package controller

import (
	"context"
	"testing"

	minecraftv1 "github.com/WangQiHao-Charlie/minecraft-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileUpdatesGameSettingAndTriggersStatefulSetRollout(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()

	if err := minecraftv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add minecraft scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}

	server := newTestServer()
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(server).Build()
	reconciler := &ServerReconciler{Client: client, Scheme: scheme}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: server.Name, Namespace: server.Namespace}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	var initialConfigMap v1.ConfigMap
	if err := client.Get(ctx, types.NamespacedName{Name: server.Name + "-config", Namespace: server.Namespace}, &initialConfigMap); err != nil {
		t.Fatalf("get configmap after first reconcile: %v", err)
	}
	if got := initialConfigMap.Data["MOTD"]; got != server.Spec.Minecraft.Motd {
		t.Fatalf("unexpected initial MOTD: got %q want %q", got, server.Spec.Minecraft.Motd)
	}

	var initialStatefulSet appsv1.StatefulSet
	if err := client.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, &initialStatefulSet); err != nil {
		t.Fatalf("get statefulset after first reconcile: %v", err)
	}
	initialHash := initialStatefulSet.Spec.Template.Annotations[podTemplateHashAnnotation]

	var initialService v1.Service
	if err := client.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, &initialService); err != nil {
		t.Fatalf("get service after first reconcile: %v", err)
	}
	if !serviceHasPort(initialService.Spec.Ports, minecraftPortName, minecraftPort) {
		t.Fatalf("expected minecraft service port %d, got %#v", minecraftPort, initialService.Spec.Ports)
	}
	if serviceHasPort(initialService.Spec.Ports, rconPortName, rconPort) {
		t.Fatalf("did not expect rcon service port before enable_rcon, got %#v", initialService.Spec.Ports)
	}

	var updatedServer minecraftv1.Server
	if err := client.Get(ctx, req.NamespacedName, &updatedServer); err != nil {
		t.Fatalf("get server for update: %v", err)
	}
	updatedServer.Spec.Minecraft.Motd = "updated motd"
	updatedServer.Spec.Minecraft.EnableRcon = true
	updatedServer.Spec.HardwareResource.CPUCount = 2000
	updatedServer.Spec.HardwareResource.MemorySize = 3072

	if err := client.Update(ctx, &updatedServer); err != nil {
		t.Fatalf("update server: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	var updatedConfigMap v1.ConfigMap
	if err := client.Get(ctx, types.NamespacedName{Name: server.Name + "-config", Namespace: server.Namespace}, &updatedConfigMap); err != nil {
		t.Fatalf("get configmap after second reconcile: %v", err)
	}
	if got := updatedConfigMap.Data["MOTD"]; got != "updated motd" {
		t.Fatalf("unexpected updated MOTD: got %q want %q", got, "updated motd")
	}
	if got := updatedConfigMap.Data["ENABLE_RCON"]; got != "TRUE" {
		t.Fatalf("unexpected ENABLE_RCON: got %q want %q", got, "TRUE")
	}

	var updatedStatefulSet appsv1.StatefulSet
	if err := client.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, &updatedStatefulSet); err != nil {
		t.Fatalf("get statefulset after second reconcile: %v", err)
	}
	updatedHash := updatedStatefulSet.Spec.Template.Annotations[podTemplateHashAnnotation]
	if initialHash == updatedHash {
		t.Fatalf("expected pod template hash to change")
	}

	container := updatedStatefulSet.Spec.Template.Spec.Containers[0]
	if got, want := container.Resources.Requests.Cpu().Cmp(resource.MustParse("2000m")), 0; got != want {
		t.Fatalf("unexpected cpu request: got %s want %s", container.Resources.Requests.Cpu().String(), "2000m")
	}
	if got, want := container.Resources.Requests.Memory().Cmp(resource.MustParse("4301Mi")), 0; got != want {
		t.Fatalf("unexpected memory request: got %s want %s", container.Resources.Requests.Memory().String(), "4301Mi")
	}
	if got := container.Image; got != "itzg/minecraft-server:21" {
		t.Fatalf("unexpected image: got %q want %q", got, "itzg/minecraft-server:21")
	}
	if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].Name != minecraftDataVolumeName || container.VolumeMounts[0].MountPath != minecraftDataMountPath {
		t.Fatalf("unexpected volume mounts: %#v", container.VolumeMounts)
	}

	var updatedService v1.Service
	if err := client.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, &updatedService); err != nil {
		t.Fatalf("get service after second reconcile: %v", err)
	}
	if !serviceHasPort(updatedService.Spec.Ports, minecraftPortName, minecraftPort) {
		t.Fatalf("expected minecraft service port %d, got %#v", minecraftPort, updatedService.Spec.Ports)
	}
	if !serviceHasPort(updatedService.Spec.Ports, rconPortName, rconPort) {
		t.Fatalf("expected rcon service port %d after enable_rcon, got %#v", rconPort, updatedService.Spec.Ports)
	}
}

func TestReconcileTogglesDebugWebDAVSidecarAndService(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()

	if err := minecraftv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add minecraft scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}

	server := newTestServer()
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(server).Build()
	reconciler := &ServerReconciler{Client: client, Scheme: scheme}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: server.Name, Namespace: server.Namespace}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile with debug disabled: %v", err)
	}

	var initialStatefulSet appsv1.StatefulSet
	if err := client.Get(ctx, req.NamespacedName, &initialStatefulSet); err != nil {
		t.Fatalf("get statefulset after initial reconcile: %v", err)
	}
	if _, ok := findContainer(initialStatefulSet.Spec.Template.Spec.Containers, debugWebDAVContainerName); ok {
		t.Fatalf("debug sidecar should not exist when debug is disabled")
	}

	var absentService v1.Service
	err := client.Get(ctx, types.NamespacedName{Name: server.Name + debugWebDAVServiceSuffix, Namespace: server.Namespace}, &absentService)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected debug service to be absent, got err=%v", err)
	}

	var updatedServer minecraftv1.Server
	if err := client.Get(ctx, req.NamespacedName, &updatedServer); err != nil {
		t.Fatalf("get server for debug update: %v", err)
	}
	updatedServer.Spec.Debug.Enabled = true
	if err := client.Update(ctx, &updatedServer); err != nil {
		t.Fatalf("enable debug: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile with debug enabled: %v", err)
	}

	var debugStatefulSet appsv1.StatefulSet
	if err := client.Get(ctx, req.NamespacedName, &debugStatefulSet); err != nil {
		t.Fatalf("get statefulset with debug enabled: %v", err)
	}
	debugContainer, ok := findContainer(debugStatefulSet.Spec.Template.Spec.Containers, debugWebDAVContainerName)
	if !ok {
		t.Fatalf("expected debug sidecar to exist")
	}
	if got, want := debugContainer.Image, "rclone/rclone:latest"; got != want {
		t.Fatalf("unexpected debug image: got %q want %q", got, want)
	}
	if len(debugContainer.VolumeMounts) != 1 || debugContainer.VolumeMounts[0].Name != minecraftDataVolumeName || debugContainer.VolumeMounts[0].MountPath != minecraftDataMountPath {
		t.Fatalf("unexpected debug volume mounts: %#v", debugContainer.VolumeMounts)
	}

	var debugService v1.Service
	if err := client.Get(ctx, types.NamespacedName{Name: server.Name + debugWebDAVServiceSuffix, Namespace: server.Namespace}, &debugService); err != nil {
		t.Fatalf("get debug service: %v", err)
	}
	if got, want := debugService.Spec.Type, v1.ServiceTypeClusterIP; got != want {
		t.Fatalf("unexpected debug service type: got %q want %q", got, want)
	}
	if len(debugService.Spec.Ports) != 1 || debugService.Spec.Ports[0].Port != debugWebDAVPort {
		t.Fatalf("unexpected debug service ports: %#v", debugService.Spec.Ports)
	}

	if err := client.Get(ctx, req.NamespacedName, &updatedServer); err != nil {
		t.Fatalf("get server for debug disable: %v", err)
	}
	updatedServer.Spec.Debug.Enabled = false
	if err := client.Update(ctx, &updatedServer); err != nil {
		t.Fatalf("disable debug: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile with debug disabled again: %v", err)
	}

	var finalStatefulSet appsv1.StatefulSet
	if err := client.Get(ctx, req.NamespacedName, &finalStatefulSet); err != nil {
		t.Fatalf("get statefulset after disabling debug: %v", err)
	}
	if _, ok := findContainer(finalStatefulSet.Spec.Template.Spec.Containers, debugWebDAVContainerName); ok {
		t.Fatalf("debug sidecar should be removed when debug is disabled")
	}

	err = client.Get(ctx, types.NamespacedName{Name: server.Name + debugWebDAVServiceSuffix, Namespace: server.Namespace}, &debugService)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected debug service to be deleted, got err=%v", err)
	}
}

func newTestServer() *minecraftv1.Server {
	return &minecraftv1.Server{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-server",
			Namespace: "default",
		},
		Spec: minecraftv1.ServerSpec{
			HardwareResource: minecraftv1.HardwareSpec{
				StorageSize:      10,
				MemorySize:       2048,
				CPUCount:         1000,
				StorageClassName: "standard",
			},
			JavaVersion: "21",
			Minecraft: minecraftv1.MinecraftConfig{
				Eula:       "TRUE",
				Type:       "PAPER",
				Version:    "1.21.1",
				Motd:       "initial motd",
				OnlineMode: true,
				LevelName:  "world",
				Modpack: minecraftv1.ModpackSource{
					Type: "HTTP",
					URL:  "https://example.invalid/modpack.zip",
				},
			},
		},
	}
}

func findContainer(containers []v1.Container, name string) (v1.Container, bool) {
	for _, container := range containers {
		if container.Name == name {
			return container, true
		}
	}

	return v1.Container{}, false
}

func serviceHasPort(ports []v1.ServicePort, name string, port int32) bool {
	for _, servicePort := range ports {
		if servicePort.Name == name && servicePort.Port == port {
			return true
		}
	}

	return false
}
