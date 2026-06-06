package mcp

import (
	"context"
	_ "embed"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed embed/container-tools.md
var containerToolsMD []byte

const containerToolsResourceURI = "netdrill://container-tools"

// ContainerToolCatalogName identifies the embedded netdrill image tool catalog.
func ContainerToolCatalogName() string {
	return "netdrill-image-tools"
}

// registerContainerTools adds MCP resources and prompts for CLIs shipped in the netdrill image.
func registerContainerTools(server *mcp.Server, _ *Deps) error {
	registerContainerToolResources(server)
	registerContainerToolPrompts(server)

	return nil
}

// registerContainerToolResources exposes the embedded tool catalog as an MCP resource.
func registerContainerToolResources(server *mcp.Server) {
	server.AddResource(&mcp.Resource{
		URI:         containerToolsResourceURI,
		Name:        "netdrill-container-tools",
		Description: "Catalog of CLIs and utilities in the netdrill image; use netdrill_pod_exec to run them",
		MIMEType:    "text/markdown",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      containerToolsResourceURI,
				MIMEType: "text/markdown",
				Text:     string(containerToolsMD),
			}},
		}, nil
	})
}

// registerContainerToolPrompts adds MCP prompts that guide agents to run image CLIs via pod exec.
func registerContainerToolPrompts(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "netdrill_prompt_aws_in_pod",
		Description: "Verify AWS credentials from inside a netdrill pod via aws-probe CLI",
	}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "Run aws-probe in a netdrill pod (IRSA via ServiceAccount)",
			Messages: []*mcp.PromptMessage{{
				Role: "user",
				Content: &mcp.TextContent{
					Text: "Read the netdrill://container-tools resource. " +
						"Create a netdrill pod with an appropriate serviceAccountName for AWS (IRSA on EKS). " +
						"Wait until Running, then netdrill_pod_exec with command [\"aws-probe\",\"whoami\"]. " +
						"Report account, ARN, and auth method. " +
						"For S3 listing use [\"aws-probe\",\"s3\",\"list-buckets\"]. " +
						"Delete the pod when done. " +
						"Do not use aws-probe mcp on the host unless the user asked for workstation AWS access.",
				},
			}},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "netdrill_prompt_https_in_pod",
		Description: "Run HTTPS probes from inside the cluster using https-wrench",
	}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "HTTPS/TLS check via https-wrench in a netdrill pod",
			Messages: []*mcp.PromptMessage{{
				Role: "user",
				Content: &mcp.TextContent{
					Text: "Read netdrill://container-tools. Create a netdrill pod, wait until Running, " +
						"then netdrill_pod_exec with https-wrench per upstream docs (config or flags). " +
						"Summarize TLS/cert results. Delete the pod when finished.",
				},
			}},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "netdrill_prompt_network_check",
		Description: "Basic connectivity check from a netdrill pod (ping/curl/doggo)",
		Arguments: []*mcp.PromptArgument{
			{Name: "target", Description: "Host, IP, or URL to test (default: 1.1.1.1)", Required: false},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		target := promptArg(req, "target")
		if target == "" {
			target = "1.1.1.1"
		}

		return &mcp.GetPromptResult{
			Description: "Connectivity check from inside the cluster",
			Messages: []*mcp.PromptMessage{{
				Role: "user",
				Content: &mcp.TextContent{
					Text: "Read netdrill://container-tools. Create a netdrill pod, wait until Running, " +
						"then netdrill_pod_exec: try ping to \"" + target + "\", " +
						"then curl or doggo/nslookup as appropriate. " +
						"For authoritative DNS use doggo with --rd=false and @tcp://NS_IP per the catalog. " +
						"Summarize reachability and DNS. Delete the pod when finished.",
				},
			}},
		}, nil
	})
}

// promptArg returns a named prompt argument from req, or "" when missing.
func promptArg(req *mcp.GetPromptRequest, name string) string {
	if req == nil || req.Params == nil || req.Params.Arguments == nil {
		return ""
	}

	return req.Params.Arguments[name]
}
