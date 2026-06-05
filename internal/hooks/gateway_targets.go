package hooks

import (
	"fmt"
	"io"
)

// printGatewayGuide prints configuration instructions for gateway-based targets
// (Continue, OpenCode, OpenClaw). No files are written.
//
// Each guide shows the user how to configure the MCP client to point at the
// Beekeeper gateway and how to retrieve the per-session auth token.
func printGatewayGuide(target string, out io.Writer) error {
	switch target {
	case TargetContinue:
		return printContinueGuide(out)
	case TargetOpenClaw:
		return printOpenClawGuide(out)
	case TargetKilo:
		return printKiloGuide(out)
	case TargetTrae:
		return printTraeGuide(out)
	default:
		return fmt.Errorf("printGatewayGuide: unknown gateway target %q", target)
	}
}

// printContinueGuide prints Continue.dev MCP configuration instructions.
// Verified schema from docs.continue.dev/customize/deep-dives/mcp.
func printContinueGuide(out io.Writer) error {
	fmt.Fprintf(out, `Beekeeper Gateway — Continue.dev Configuration
===============================================

No file has been written. Add the following to your Continue config:

  ~/.continue/config.yaml  (or  <project>/.continue/config.yaml):

    mcpServers:
      - name: Beekeeper Gateway
        type: streamable-http
        url: http://127.0.0.1:7837/mcp
        env:
          BEEKEEPER_GATEWAY_TOKEN: "${BEEKEEPER_GATEWAY_TOKEN}"

Set the auth token in your shell environment before starting Continue:

  export BEEKEEPER_GATEWAY_TOKEN=$(beekeeper gateway token)

Get the current token:

  beekeeper gateway token

`)
	return nil
}

// printOpenCodeGuide prints OpenCode MCP gateway configuration instructions.
// OpenCode also supports a JS plugin installer (installOpenCodePlugin) that
// provides pre-exec blocking via tool.execute.before; this guide is kept as a
// fallback reference for users who prefer the MCP gateway path.
// Verified schema from opencode.ai/docs/mcp-servers/.
func printOpenCodeGuide(out io.Writer) error {
	fmt.Fprintf(out, `Beekeeper Gateway — OpenCode Configuration
==========================================

No file has been written. Add the following to your OpenCode config:

  opencode.json  (project-level  or  ~/.config/opencode/opencode.json):

    {
      "$schema": "https://opencode.ai/config.json",
      "mcp": {
        "beekeeper": {
          "type": "remote",
          "url": "http://127.0.0.1:7837/mcp",
          "enabled": true,
          "headers": {
            "Authorization": "Bearer {env:BEEKEEPER_GATEWAY_TOKEN}"
          }
        }
      }
    }

Set the auth token in your shell environment before starting OpenCode:

  export BEEKEEPER_GATEWAY_TOKEN=$(beekeeper gateway token)

Get the current token:

  beekeeper gateway token

`)
	return nil
}

// printOpenClawGuide prints OpenClaw MCP configuration instructions.
// Verified schema from docs.openclaw.ai/cli/mcp.
func printOpenClawGuide(out io.Writer) error {
	fmt.Fprintf(out, `Beekeeper Gateway — OpenClaw Configuration
==========================================

No file has been written. Add the following to your OpenClaw config:

  openclaw.json  (or  ~/.openclaw/config.json):

    {
      "mcp": {
        "servers": {
          "beekeeper": {
            "url": "http://127.0.0.1:7837/mcp",
            "transport": "streamable-http",
            "headers": {
              "Authorization": "Bearer <token>"
            }
          }
        }
      }
    }

Replace <token> with the output of:

  beekeeper gateway token

Get the current token:

  beekeeper gateway token

`)
	return nil
}

// printKiloGuide and printTraeGuide are defined in kilo_trae.go.
// They print MCP gateway routing guides for harnesses that have no pre-exec
// hook (Kilo FR #5827; Trae no programmatic hook). Both guides prominently
// state that native tools are UNGUARDED.
