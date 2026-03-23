package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServerSpec defines the desired state of Server
type ServerSpec struct {
	// Server Name is the display name of the server
	ServerName string `json:"server_name,omitempty"`

	// Server Unique ID
	ServerId string `json:"server_id,omitempty"`

	// Hardware resources allocation
	HardwareResource HardwareSpec `json:"hardware"`

	// Java version
	JavaVersion string `json:"java_version,omitempty"`

	// Minecraft-specific config
	Minecraft MinecraftConfig `json:"minecraft,omitempty"`

	// Snapshot Policy
	SnapshotPolicy SnapshotPolicy `json:"snapshotPolicy,omitempty"`

	// Debug settings for the server pod.
	// +optional
	Debug DebugSpec `json:"debug,omitempty"`
}

type DebugSpec struct {
	// Enable debug sidecar and its Service.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

type MinecraftConfig struct {
	// +kubebuilder:validation:Enum=TRUE;FALSE
	Eula string `json:"eula"`

	// +kubebuilder:validation:Enum=VANILLA;PAPER;FORGE;FABRIC;SPIGOT
	Type string `json:"type"`

	// Minecraft server version, e.g., "1.21.1"
	// +optional
	Version string `json:"version,omitempty"`

	// Enable RCON
	// +optional
	EnableRcon bool `json:"enable_rcon,omitempty"`

	// +optional
	RconPassword string `json:"rcon_password,omitempty"`

	// +optional
	Motd string `json:"motd,omitempty"`

	// +optional
	Difficulty string `json:"difficulty,omitempty"`

	// +optional
	OnlineMode bool `json:"online_mode,omitempty"`

	// +optional
	LevelName string `json:"level_name,omitempty"`

	// modpack source
	Modpack ModpackSource `json:"modpack,omitempty"`

	// Optional: Quote external ConfigMap
	// +optional
	ConfigRef *corev1.LocalObjectReference `json:"configRef,omitempty"`
}

type ModpackSource struct {
	// +kubebuilder:validation:Enum=S3;HTTP;Local
	Type       string `json:"type"`
	URL        string `json:"url"`
	Checksum   string `json:"checksum,omitempty"`
	AutoUpdate bool   `json:"auto_update,omitempty"`
}

type HardwareSpec struct {
	// Instance PVC Size
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=51200
	StorageSize int `json:"storage_size"`

	// Adopt and mount an existing PVC instead of creating a StatefulSet volumeClaimTemplate.
	// +optional
	ExistingClaimName string `json:"existing_claim_name,omitempty"`

	// Instance Memory Limit (MB)
	// +kubebuilder:validation:Minimum=512
	// +kubebuilder:validation:Maximum=32768
	MemorySize int `json:"memory_size"`

	// Instance CPU limit	(mCPU)
	// +kubebuilder:validation:Minimum=500
	// +kubebuilder:validation:Maximum=16000
	CPUCount int `json:"cpu_count"`

	// Schedule Tag
	// +optional
	HighFrequency bool `json:"high_freq,omitempty"`

	// +optional
	StorageClassName string `json:"storage_class,omitempty"`

	IOThrottle int `json:"io_throttle,omitempty"`
}

type RuntimeSpec struct {
	// +kubebuilder:validation:Enum=container;microvm
	Type string `json:"type"`

	Image string `json:"image,omitempty"`

	// Kernel image
	KernelImage string `json:"kernel_image,omitempty"`

	Entrypoint []string `json:"entrypoint,omitempty"`

	Env map[string]string `json:"env,omitempty"`

	Extra map[string]string `json:"extra,omitempty"`
}

// ServerStatus defines the observed state of Server.
type ServerStatus struct {
	// conditions represent the current state of the Server resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Server is the Schema for the servers API
type Server struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerSpec   `json:"spec"`
	Status ServerStatus `json:"status,omitempty"`
}

type SnapshotPolicy struct {
	// Enable Snapshot
	Enabled bool `json:"enabled"`

	// Enable Restore
	AutoRestore bool `json:"auto_restore"`

	// Automatic backup snapshot
	AutoBackup bool `json:"auto_backup"`

	// e.g. s3://minecraft-snapshots/survival-latest.snap
	StorageURL string `json:"storage_url,omitempty"`

	LastSnapshotID string `json:"last_snapshot_id,omitempty"`
}

// +kubebuilder:object:root=true

// ServerList contains a list of Server
type ServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Server `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Server{}, &ServerList{})
}
