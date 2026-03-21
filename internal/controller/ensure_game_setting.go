package controller

import (
	"context"
	"fmt"

	minecraftv1 "github.com/WangQiHao-Charlie/minecraft-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func buildGameSettingData(server *minecraftv1.Server) map[string]string {
	eula := server.Spec.Minecraft.Eula
	if eula == "" {
		eula = "TRUE"
	}

	return map[string]string{
		"EULA":          eula,
		"TYPE":          server.Spec.Minecraft.Type,
		"VERSION":       server.Spec.Minecraft.Version,
		"MEMORY":        fmt.Sprintf("%dM", server.Spec.HardwareResource.MemorySize),
		"MOTD":          server.Spec.Minecraft.Motd,
		"MODPACK":       server.Spec.Minecraft.Modpack.URL,
		"ENABLE_RCON":   boolEnv(server.Spec.Minecraft.EnableRcon),
		"RCON_PASSWORD": server.Spec.Minecraft.RconPassword,
		"DIFFICULTY":    server.Spec.Minecraft.Difficulty,
		"ONLINE_MODE":   boolEnv(server.Spec.Minecraft.OnlineMode),
		"LEVEL":         server.Spec.Minecraft.LevelName,
	}
}

func boolEnv(value bool) string {
	if value {
		return "TRUE"
	}

	return "FALSE"
}

func (r *ServerReconciler) ensureGameSetting(ctx context.Context, server *minecraftv1.Server, _ *ServerReconciler) error {
	data := buildGameSettingData(server)
	name := types.NamespacedName{
		Name:      server.Name + "-config",
		Namespace: server.Namespace,
	}

	var configMap v1.ConfigMap
	err := r.Get(ctx, name, &configMap)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		configMap = v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
			},
			Data: data,
		}

		if err := ctrl.SetControllerReference(server, &configMap, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, &configMap)
	}

	if equality.Semantic.DeepEqual(configMap.Data, data) {
		return nil
	}

	configMap.Data = data
	return r.Update(ctx, &configMap)
}
