package cmd

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	dryRunNone   = "none"
	dryRunClient = "client"
	outputYAML   = "yaml"
	outputJSON   = "json"
)

// validateDryRunOutput validates --dry-run and -o/--output flag combinations.
// When dry-run=client is active and output is empty, it defaults output to "yaml".
func validateDryRunOutput(dryRun, output string) (isDryRun bool, format string, err error) {
	switch dryRun {
	case "", dryRunNone:
		isDryRun = false
	case dryRunClient:
		isDryRun = true
	default:
		return false, "", fmt.Errorf("invalid --dry-run value %q: must be %q or %q", dryRun, dryRunNone, dryRunClient)
	}

	if !isDryRun {
		if output != "" {
			return false, "", fmt.Errorf("--output (%q) is only supported with --dry-run=client", output)
		}

		return false, "", nil
	}

	if output == "" {
		return true, outputYAML, nil
	}

	switch output {
	case outputYAML, outputJSON:
		return true, output, nil
	default:
		return false, "", fmt.Errorf("invalid --output value %q: must be %q or %q", output, outputYAML, outputJSON)
	}
}

// printDryRunObject serializes a Kubernetes runtime.Object as YAML or JSON to out,
// populating TypeMeta (apiVersion and kind) via the client-go scheme.
func printDryRunObject(out io.Writer, obj runtime.Object, format string) error {
	var basePrinter printers.ResourcePrinter

	switch format {
	case outputYAML:
		basePrinter = &printers.YAMLPrinter{}
	case outputJSON:
		basePrinter = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}

	printer := printers.NewTypeSetter(scheme.Scheme).ToPrinter(basePrinter)

	return printer.PrintObj(obj, out)
}
