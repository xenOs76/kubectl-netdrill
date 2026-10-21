package k8s

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// BuildResourceRequirements parses CPU and memory request/limit strings into corev1.ResourceRequirements.
func BuildResourceRequirements(
	cpuRequest, memoryRequest, cpuLimit, memoryLimit string,
) (corev1.ResourceRequirements, error) {
	var err error

	resources := corev1.ResourceRequirements{}

	resources.Requests, err = parseResource(cpuRequest, "cpu-request", resources.Requests, corev1.ResourceCPU)
	if err != nil {
		return resources, err
	}

	resources.Requests, err = parseResource(memoryRequest, "memory-request", resources.Requests, corev1.ResourceMemory)
	if err != nil {
		return resources, err
	}

	resources.Limits, err = parseResource(cpuLimit, "cpu-limit", resources.Limits, corev1.ResourceCPU)
	if err != nil {
		return resources, err
	}

	resources.Limits, err = parseResource(memoryLimit, "memory-limit", resources.Limits, corev1.ResourceMemory)
	if err != nil {
		return resources, err
	}

	return resources, nil
}

// parseResource parses a string value into a Quantity and adds it to the ResourceList.
func parseResource(
	val, name string,
	list corev1.ResourceList,
	resName corev1.ResourceName,
) (corev1.ResourceList, error) {
	if val == "" {
		return list, nil
	}

	q, err := resource.ParseQuantity(val)
	if err != nil {
		return list, fmt.Errorf("invalid %s %q: %w", name, val, err)
	}

	if list == nil {
		list = make(corev1.ResourceList)
	}

	list[resName] = q

	return list, nil
}
