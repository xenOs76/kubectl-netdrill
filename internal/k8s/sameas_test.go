package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestParseSameAsTarget_ValidSimple(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		defaultKind      string
		defaultNamespace string
		want             TargetInfo
	}{
		{
			name:             "bare name with default kind and namespace",
			target:           "my-workload",
			defaultKind:      "pod",
			defaultNamespace: "default",
			want: TargetInfo{
				Namespace: "default",
				Kind:      "pod",
				Name:      "my-workload",
			},
		},
		{
			name:             "kind and name with pod",
			target:           "pod/my-pod",
			defaultKind:      "deployment",
			defaultNamespace: "prod",
			want: TargetInfo{
				Namespace: "prod",
				Kind:      "pod",
				Name:      "my-pod",
			},
		},
		{
			name:             "kind and name with deployment",
			target:           "deployment/api-service",
			defaultKind:      "pod",
			defaultNamespace: "prod",
			want: TargetInfo{
				Namespace: "prod",
				Kind:      "deployment",
				Name:      "api-service",
			},
		},
		{
			name:             "short kind aliases",
			target:           "deploy/api",
			defaultKind:      "pod",
			defaultNamespace: "default",
			want: TargetInfo{
				Namespace: "default",
				Kind:      "deployment",
				Name:      "api",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSameAsTarget(tt.target, tt.defaultKind, tt.defaultNamespace)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseSameAsTarget_ValidScoped(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		defaultKind      string
		defaultNamespace string
		want             TargetInfo
	}{
		{
			name:             "namespace and name",
			target:           "staging/worker",
			defaultKind:      "pod",
			defaultNamespace: "default",
			want: TargetInfo{
				Namespace: "staging",
				Kind:      "pod",
				Name:      "worker",
			},
		},
		{
			name:             "namespace, kind, and name",
			target:           "custom-ns/deployment/backend",
			defaultKind:      "pod",
			defaultNamespace: "default",
			want: TargetInfo{
				Namespace: "custom-ns",
				Kind:      "deployment",
				Name:      "backend",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSameAsTarget(tt.target, tt.defaultKind, tt.defaultNamespace)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseSameAsTarget_Errors(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		defaultKind      string
		defaultNamespace string
		errContains      string
	}{
		{
			name:             "empty target error",
			target:           "   ",
			defaultKind:      "pod",
			defaultNamespace: "default",
			errContains:      "cannot be empty",
		},
		{
			name:             "unsupported kind with 3 parts",
			target:           "default/service/my-svc",
			defaultKind:      "pod",
			defaultNamespace: "default",
			errContains:      "unsupported kind",
		},
		{
			name:             "unsupported default kind",
			target:           "my-svc",
			defaultKind:      "service",
			defaultNamespace: "default",
			errContains:      "unsupported kind",
		},
		{
			name:             "too many segments",
			target:           "a/b/c/d",
			defaultKind:      "pod",
			defaultNamespace: "default",
			errContains:      "invalid target format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSameAsTarget(tt.target, tt.defaultKind, tt.defaultNamespace)
			require.Error(t, err)

			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}
		})
	}
}

func TestSelectContainer(t *testing.T) {
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "app"},
			{Name: "sidecar"},
		},
	}

	t.Run("default first container", func(t *testing.T) {
		c, err := SelectContainer(spec, "")
		require.NoError(t, err)
		assert.Equal(t, "app", c.Name)
	})

	t.Run("explicit container found", func(t *testing.T) {
		c, err := SelectContainer(spec, "sidecar")
		require.NoError(t, err)
		assert.Equal(t, "sidecar", c.Name)
	})

	t.Run("container not found", func(t *testing.T) {
		c, err := SelectContainer(spec, "nonexistent")
		require.Error(t, err)
		assert.Nil(t, c)
		assert.Contains(t, err.Error(), `container "nonexistent" not found`)
		assert.Contains(t, err.Error(), "app, sidecar")
	})

	t.Run("empty containers error", func(t *testing.T) {
		emptySpec := &corev1.PodSpec{}
		c, err := SelectContainer(emptySpec, "")
		require.Error(t, err)
		assert.Nil(t, c)
	})
}

func TestFilterLabelsAndAnnotations(t *testing.T) {
	labels := map[string]string{
		"app":                                "web",
		"env":                                "prod",
		"pod-template-hash":                  "6b9df949b4",
		"controller-revision-hash":           "12345",
		"statefulset.kubernetes.io/pod-name": "web-0",
		"kubectl-netdrill.io/managed":        "true",
	}

	filteredLabels := FilterLabels(labels)
	assert.Equal(t, map[string]string{
		"app": "web",
		"env": "prod",
	}, filteredLabels)

	annotations := map[string]string{
		"team": "sre",
		"kubectl.kubernetes.io/last-applied-configuration": "{}",
		"deployment.kubernetes.io/revision":                "2",
	}

	filteredAnnotations := FilterAnnotations(annotations)
	assert.Equal(t, map[string]string{
		"team": "sre",
	}, filteredAnnotations)
}

func TestFilterVolumesAndMounts(t *testing.T) {
	spec := &corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "pvc-data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-claim"},
				},
			},
			{
				Name: "kube-api-access-abcde",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{},
				},
			},
			{
				Name: "aws-iam-token",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{},
				},
			},
			{
				Name: "config-vol",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "app-config"},
					},
				},
			},
			{
				Name: "secret-vol",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: "app-secret"},
				},
			},
		},
	}

	container := &corev1.Container{
		Name: "app",
		VolumeMounts: []corev1.VolumeMount{
			{Name: "pvc-data", MountPath: "/var/data"},
			{Name: "config-vol", MountPath: "/etc/config"},
			{Name: "secret-vol", MountPath: "/etc/secret"},
		},
	}

	vols, mounts := FilterVolumesAndMounts(spec, container)

	require.Len(t, vols, 2)
	assert.Equal(t, "config-vol", vols[0].Name)
	assert.Equal(t, "secret-vol", vols[1].Name)

	require.Len(t, mounts, 2)
	assert.Equal(t, "config-vol", mounts[0].Name)
	assert.Equal(t, "secret-vol", mounts[1].Name)
}

func newTestSameAsWorkload() *SameAsWorkload {
	cpuReq := resource.MustParse("200m")
	memReq := resource.MustParse("256Mi")
	cpuLim := resource.MustParse("500m")
	memLim := resource.MustParse("512Mi")
	user := int64(1000)

	return &SameAsWorkload{
		Target: TargetInfo{Namespace: "default", Kind: "pod", Name: "source-pod"},
		Labels: map[string]string{
			"app":               "source",
			"tier":              "backend",
			"pod-template-hash": "bad-hash",
		},
		Annotations: map[string]string{
			"custom.io/note": "test-annotation",
		},
		PodSpec: corev1.PodSpec{
			ServiceAccountName: "source-sa",
			HostNetwork:        true,
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
				"node-role":        "worker",
			},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{},
			},
			Tolerations: []corev1.Toleration{
				{Key: "special-taint", Operator: corev1.TolerationOpExists},
			},
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule},
			},
			DNSConfig: &corev1.PodDNSConfig{
				Nameservers: []string{"1.1.1.1"},
			},
			DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
			PriorityClassName: "high-priority",
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: &user,
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					Ports: []corev1.ContainerPort{
						{ContainerPort: 8080},
						{ContainerPort: 9090},
					},
					Env: []corev1.EnvVar{
						{Name: "DB_HOST", Value: "postgres.local"},
						{Name: "LOG_LEVEL", Value: "info"},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    cpuReq,
							corev1.ResourceMemory: memReq,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    cpuLim,
							corev1.ResourceMemory: memLim,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: &user,
					},
				},
			},
		},
	}
}

func TestApplySameAsWorkload_ClonesAllSettings(t *testing.T) {
	workload := newTestSameAsWorkload()
	cfg := netdrill.PodConfig{
		Namespace: "default",
		PodName:   "netdrill",
		Image:     "ghcr.io/xenos76/netdrill:latest",
	}

	err := ApplySameAsWorkload(workload, "", SameAsOverrides{}, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "ghcr.io/xenos76/netdrill:latest", cfg.Image)
	assert.Equal(t, "source-sa", cfg.ServiceAccount)
	assert.True(t, cfg.HostNetwork)
	assert.Equal(t, map[string]string{
		"kubernetes.io/os": "linux",
		"node-role":        "worker",
	}, cfg.NodeSelector)
	assert.Equal(t, map[string]string{"app": "source", "tier": "backend"}, cfg.Labels)
	assert.Equal(t, map[string]string{"custom.io/note": "test-annotation"}, cfg.Annotations)
	assert.Equal(t, []int32{8080, 9090}, cfg.Ports)

	require.Len(t, cfg.RawEnvVars, 2)
	assert.Equal(t, "DB_HOST", cfg.RawEnvVars[0].Name)
	assert.Equal(t, "LOG_LEVEL", cfg.RawEnvVars[1].Name)

	assert.Equal(t, "200m", cfg.CPURequest)
	assert.Equal(t, "256Mi", cfg.MemoryRequest)
	assert.Equal(t, "500m", cfg.CPULimit)
	assert.Equal(t, "512Mi", cfg.MemoryLimit)

	assert.NotNil(t, cfg.Affinity)
	require.Len(t, cfg.Tolerations, 1)
	require.Len(t, cfg.TopologySpreadConstraints, 1)
	assert.NotNil(t, cfg.DNSConfig)
	assert.Equal(t, corev1.DNSClusterFirstWithHostNet, cfg.DNSPolicy)
	assert.Equal(t, "high-priority", cfg.PriorityClassName)
	assert.NotNil(t, cfg.SecurityContext)
	assert.NotNil(t, cfg.ContainerSecurityContext)
}

func TestApplySameAsWorkload_ExplicitOverrides(t *testing.T) {
	workload := newTestSameAsWorkload()
	cfg := netdrill.PodConfig{
		Namespace:      "default",
		PodName:        "netdrill",
		Image:          "ghcr.io/xenos76/netdrill:latest",
		ServiceAccount: "custom-sa",
		HostNetwork:    false,
		NodeSelector:   map[string]string{"node-role": "custom-worker", "new-key": "val"},
		Labels:         map[string]string{"env": "debug", "app": "overridden"},
		Ports:          []int32{443},
		EnvVars:        map[string]string{"LOG_LEVEL": "debug", "NEW_VAR": "true"},
		CPURequest:     "100m",
		MemoryRequest:  "128Mi",
		CPULimit:       "1",
		MemoryLimit:    "1Gi",
	}

	overrides := SameAsOverrides{
		HasServiceAccount: true,
		HasHostNetwork:    true,
		HasNodeSelector:   true,
		HasLabels:         true,
		HasPorts:          true,
		HasEnv:            true,
		HasCPURequest:     true,
		HasMemoryRequest:  true,
		HasCPULimit:       true,
		HasMemoryLimit:    true,
	}

	err := ApplySameAsWorkload(workload, "", overrides, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "custom-sa", cfg.ServiceAccount)
	assert.False(t, cfg.HostNetwork)
	assert.Equal(t, []int32{443}, cfg.Ports)
	assert.Equal(t, "100m", cfg.CPURequest)
	assert.Equal(t, "128Mi", cfg.MemoryRequest)
	assert.Equal(t, "1", cfg.CPULimit)
	assert.Equal(t, "1Gi", cfg.MemoryLimit)
	assert.Equal(t, map[string]string{
		"kubernetes.io/os": "linux",
		"node-role":        "custom-worker",
		"new-key":          "val",
	}, cfg.NodeSelector)
	assert.Equal(t, map[string]string{
		"app":  "overridden",
		"tier": "backend",
		"env":  "debug",
	}, cfg.Labels)

	foundLogLevel := false
	foundNewVar := false

	for _, e := range cfg.RawEnvVars {
		if e.Name == "LOG_LEVEL" {
			assert.Equal(t, "debug", e.Value)

			foundLogLevel = true
		}

		if e.Name == "NEW_VAR" {
			assert.Equal(t, "true", e.Value)

			foundNewVar = true
		}
	}

	assert.True(t, foundLogLevel)
	assert.True(t, foundNewVar)
}

func TestFetchSameAsWorkload_FakeClient(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels:    map[string]string{"tier": "frontend"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main"}},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deploy",
				Namespace: "prod",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"tier": "api"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "web"}},
					},
				},
			},
		},
	)

	t.Run("fetch pod", func(t *testing.T) {
		w, err := FetchSameAsWorkload(ctx, client, TargetInfo{Namespace: "default", Kind: "pod", Name: "test-pod"})
		require.NoError(t, err)
		assert.Equal(t, "frontend", w.Labels["tier"])
		assert.Equal(t, "main", w.PodSpec.Containers[0].Name)
	})

	t.Run("fetch deployment", func(t *testing.T) {
		target := TargetInfo{
			Namespace: "prod",
			Kind:      "deployment",
			Name:      "test-deploy",
		}
		w, err := FetchSameAsWorkload(ctx, client, target)
		require.NoError(t, err)
		assert.Equal(t, "api", w.Labels["tier"])
		assert.Equal(t, "web", w.PodSpec.Containers[0].Name)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := FetchSameAsWorkload(ctx, client, TargetInfo{Namespace: "default", Kind: "pod", Name: "missing"})
		require.Error(t, err)
	})
}
