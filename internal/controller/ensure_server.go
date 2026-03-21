package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	minecraftv1 "github.com/WangQiHao-Charlie/minecraft-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	podTemplateHashAnnotation = "minecraft.charlie-cloud.me/template-hash"
	minecraftDataVolumeName   = "minecraft-data"
	minecraftDataMountPath    = "/data"
	minecraftContainerName    = "minecraft"
	minecraftPortName         = "minecraft"
	minecraftPort             = 25565
	rconPortName              = "rcon"
	rconPort                  = 25575
	debugWebDAVContainerName  = "debug-webdav"
	debugWebDAVServiceSuffix  = "-debug"
	debugWebDAVPortName       = "webdav"
	debugWebDAVPort           = 8080
)

func (r *ServerReconciler) ensureServer(ctx context.Context, server *minecraftv1.Server, _ *ServerReconciler) error {
	labels := map[string]string{
		"app": server.Name,
	}
	templateHash, err := buildPodTemplateHash(server)
	if err != nil {
		return err
	}
	desiredContainers := buildPodContainers(server)

	var statefulset appsv1.StatefulSet
	err = r.Get(ctx, types.NamespacedName{
		Name:      server.Name,
		Namespace: server.Namespace,
	}, &statefulset)

	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		newGameServer := appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      server.Name,
				Namespace: server.Namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType},
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
						Annotations: map[string]string{
							podTemplateHashAnnotation: templateHash,
						},
					},
					Spec: v1.PodSpec{
						Containers: desiredContainers,
					},
				},
				VolumeClaimTemplates: []v1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{Name: minecraftDataVolumeName},
						Spec: v1.PersistentVolumeClaimSpec{
							StorageClassName: &server.Spec.HardwareResource.StorageClassName,
							AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
							Resources: v1.VolumeResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dMi", server.Spec.HardwareResource.StorageSize)),
								},
							},
						},
					},
				},
			},
		}

		if err := ctrl.SetControllerReference(server, &newGameServer, r.Scheme); err != nil {
			return err
		}

		if err := r.Create(ctx, &newGameServer); err != nil {
			return err
		}

		statefulset = newGameServer
	}

	updated := false

	if statefulset.Spec.Template.Labels == nil {
		statefulset.Spec.Template.Labels = map[string]string{}
	}
	for key, value := range labels {
		if statefulset.Spec.Template.Labels[key] != value {
			statefulset.Spec.Template.Labels[key] = value
			updated = true
		}
	}

	if statefulset.Spec.Template.Annotations == nil {
		statefulset.Spec.Template.Annotations = map[string]string{}
	}
	if statefulset.Spec.Template.Annotations[podTemplateHashAnnotation] != templateHash {
		statefulset.Spec.Template.Annotations[podTemplateHashAnnotation] = templateHash
		updated = true
	}

	containers, containerUpdated := reconcileManagedContainers(statefulset.Spec.Template.Spec.Containers, desiredContainers)
	if containerUpdated {
		statefulset.Spec.Template.Spec.Containers = containers
		updated = true
	}

	if updated {
		if err := r.Update(ctx, &statefulset); err != nil {
			return err
		}
	}

	if err := r.ensureServerService(ctx, server, labels); err != nil {
		return err
	}

	return r.ensureDebugService(ctx, server, labels)
}

func buildPodContainers(server *minecraftv1.Server) []v1.Container {
	containers := []v1.Container{buildMinecraftContainer(server)}
	if server.Spec.Debug.Enabled {
		containers = append(containers, buildDebugWebDAVContainer())
	}

	return containers
}

func buildMinecraftContainer(server *minecraftv1.Server) v1.Container {
	return v1.Container{
		Name:            minecraftContainerName,
		Image:           fmt.Sprintf("itzg/minecraft-server:%s", server.Spec.JavaVersion),
		ImagePullPolicy: "IfNotPresent",
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", server.Spec.HardwareResource.CPUCount)),
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", int64(math.Ceil(float64(server.Spec.HardwareResource.MemorySize)*1.4)))),
			},
		},
		EnvFrom: []v1.EnvFromSource{
			{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: server.Name + "-config",
					},
				},
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      minecraftDataVolumeName,
				MountPath: minecraftDataMountPath,
			},
		},
	}
}

func buildDebugWebDAVContainer() v1.Container {
	return v1.Container{
		Name:            debugWebDAVContainerName,
		Image:           "rclone/rclone:latest",
		ImagePullPolicy: "IfNotPresent",
		Command:         []string{"rclone"},
		Args: []string{
			"serve",
			"webdav",
			minecraftDataMountPath,
			fmt.Sprintf("--addr=:%d", debugWebDAVPort),
		},
		Ports: []v1.ContainerPort{
			{
				Name:          debugWebDAVPortName,
				ContainerPort: debugWebDAVPort,
				Protocol:      v1.ProtocolTCP,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      minecraftDataVolumeName,
				MountPath: minecraftDataMountPath,
			},
		},
	}
}

func buildPodTemplateHash(server *minecraftv1.Server) (string, error) {
	payload := struct {
		GameSetting  map[string]string `json:"gameSetting"`
		JavaVersion  string            `json:"javaVersion"`
		CPUCount     int               `json:"cpuCount"`
		MemorySize   int               `json:"memorySize"`
		DebugEnabled bool              `json:"debugEnabled"`
	}{
		GameSetting:  buildGameSettingData(server),
		JavaVersion:  server.Spec.JavaVersion,
		CPUCount:     server.Spec.HardwareResource.CPUCount,
		MemorySize:   server.Spec.HardwareResource.MemorySize,
		DebugEnabled: server.Spec.Debug.Enabled,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func reconcileManagedContainers(existing []v1.Container, desired []v1.Container) ([]v1.Container, bool) {
	desiredByName := make(map[string]v1.Container, len(desired))
	for _, container := range desired {
		desiredByName[container.Name] = container
	}

	managedNames := map[string]struct{}{
		minecraftContainerName:   {},
		debugWebDAVContainerName: {},
	}

	seen := make(map[string]struct{}, len(desired))
	result := make([]v1.Container, 0, len(existing)+len(desired))
	updated := false

	for _, container := range existing {
		if _, managed := managedNames[container.Name]; !managed {
			result = append(result, container)
			continue
		}

		desiredContainer, ok := desiredByName[container.Name]
		if !ok {
			updated = true
			continue
		}

		seen[container.Name] = struct{}{}
		if equality.Semantic.DeepEqual(container, desiredContainer) {
			result = append(result, container)
			continue
		}

		result = append(result, desiredContainer)
		updated = true
	}

	for _, container := range desired {
		if _, ok := seen[container.Name]; ok {
			continue
		}

		result = append(result, container)
		updated = true
	}

	return result, updated
}

func (r *ServerReconciler) ensureServerService(ctx context.Context, server *minecraftv1.Server, labels map[string]string) error {
	return r.ensureService(ctx, server, buildMinecraftService(server, labels))
}

func (r *ServerReconciler) ensureDebugService(ctx context.Context, server *minecraftv1.Server, labels map[string]string) error {
	if !server.Spec.Debug.Enabled {
		return r.deleteServiceIfExists(ctx, types.NamespacedName{
			Name:      server.Name + debugWebDAVServiceSuffix,
			Namespace: server.Namespace,
		})
	}

	return r.ensureService(ctx, server, buildDebugService(server, labels))
}

func buildMinecraftService(server *minecraftv1.Server, labels map[string]string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Annotations: map[string]string{
				"mc-router.itzg.me/externalServerName": fmt.Sprintf("server-%s.minecraft.charlie-cloud.me", server.Name),
			},
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
			Ports:    buildMinecraftServicePorts(server),
		},
	}
}

func buildMinecraftServicePorts(server *minecraftv1.Server) []v1.ServicePort {
	ports := []v1.ServicePort{
		{
			Name:       minecraftPortName,
			Port:       minecraftPort,
			TargetPort: intstr.FromInt(minecraftPort),
		},
	}

	if server.Spec.Minecraft.EnableRcon {
		ports = append(ports, v1.ServicePort{
			Name:       rconPortName,
			Port:       rconPort,
			TargetPort: intstr.FromInt(rconPort),
		})
	}

	return ports
}

func buildDebugService(server *minecraftv1.Server, labels map[string]string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name + debugWebDAVServiceSuffix,
			Namespace: server.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []v1.ServicePort{
				{
					Name:       debugWebDAVPortName,
					Port:       debugWebDAVPort,
					TargetPort: intstr.FromString(debugWebDAVPortName),
				},
			},
		},
	}
}

func (r *ServerReconciler) ensureService(ctx context.Context, server *minecraftv1.Server, desired *v1.Service) error {
	var service v1.Service
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, &service)
	if err == nil {
		updated := false
		if !equality.Semantic.DeepEqual(service.Annotations, desired.Annotations) {
			service.Annotations = desired.Annotations
			updated = true
		}
		if !equality.Semantic.DeepEqual(service.Labels, desired.Labels) {
			service.Labels = desired.Labels
			updated = true
		}
		if !equality.Semantic.DeepEqual(service.Spec.Selector, desired.Spec.Selector) {
			service.Spec.Selector = desired.Spec.Selector
			updated = true
		}
		if !equality.Semantic.DeepEqual(service.Spec.Ports, desired.Spec.Ports) {
			service.Spec.Ports = desired.Spec.Ports
			updated = true
		}
		if service.Spec.Type != desired.Spec.Type {
			service.Spec.Type = desired.Spec.Type
			updated = true
		}
		if !updated {
			return nil
		}

		return r.Update(ctx, &service)
	}
	if !errors.IsNotFound(err) {
		return err
	}

	if err := ctrl.SetControllerReference(server, desired, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, desired)
}

func (r *ServerReconciler) deleteServiceIfExists(ctx context.Context, name types.NamespacedName) error {
	var service v1.Service
	if err := r.Get(ctx, name, &service); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return r.Delete(ctx, &service)
}
