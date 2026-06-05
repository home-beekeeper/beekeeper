package hooks

import (
	"fmt"
	"io"
)

// printKiloGuide prints Kilo MCP gateway configuration instructions.
//
// IMPORTANT: Kilo has no external pre-exec hook (open feature request #5827).
// Native built-in tools (Bash, file read/write, shell commands) are UNGUARDED —
// Beekeeper cannot intercept them via the hook path. Only MCP tools routed
// through the Beekeeper gateway are intercepted.
//
// Config location: ~/.config/kilo/kilo.json
func printKiloGuide(out io.Writer) error {
	fmt.Fprintf(out, `Beekeeper Gateway — Kilo Configuration
=======================================

IMPORTANT: Kilo does not support external pre-exec hooks (open FR #5827).
Kilo's native built-in tools (Bash, file operations, shell commands) are
UNGUARDED — Beekeeper cannot intercept them via the hook path. Only MCP tools
can be intercepted by routing them through the Beekeeper MCP gateway.

To intercept MCP tools, add the Beekeeper gateway to your kilo.json:

  kilo.json  (project-level  or  ~/.config/kilo/kilo.json):

    {
      "mcp": {
        "beekeeper": {
          "type": "remote",
          "url": "http://127.0.0.1:7837/mcp",
          "headers": {
            "Authorization": "Bearer {env:BEEKEEPER_GATEWAY_TOKEN}"
          }
        }
      }
    }

Set the auth token in your shell environment before starting Kilo:

  export BEEKEEPER_GATEWAY_TOKEN=$(beekeeper gateway token)

Get the current token:

  beekeeper gateway token

Coverage summary:
  - MCP tools:          INTERCEPTED (via MCP gateway)
  - Native Bash/file tools: UNGUARDED (no pre-exec hook available upstream)

`)
	return nil
}

// printTraeGuide prints Trae MCP gateway configuration instructions.
//
// IMPORTANT: Trae has no programmatic pre-exec hook. Native commands are gated
// only by Trae's interactive "Auto-run & security" UI setting. Only MCP tools
// routed through the Beekeeper gateway can be intercepted by Beekeeper.
//
// Config location: ~/.trae/mcp.json
func printTraeGuide(out io.Writer) error {
	fmt.Fprintf(out, `Beekeeper Gateway — Trae Configuration
=======================================

IMPORTANT: Trae does not support programmatic pre-exec hooks. Native commands
are gated only by Trae's interactive "Auto-run & security" UI — Beekeeper
cannot intercept them. Native tools (shell commands, file operations) are
UNGUARDED. Only MCP tools can be intercepted via the Beekeeper MCP gateway.

To intercept MCP tools, add the Beekeeper gateway to your Trae MCP config:

  ~/.trae/mcp.json:

    {
      "mcpServers": {
        "beekeeper": {
          "type": "streamable-http",
          "url": "http://127.0.0.1:7837/mcp",
          "headers": {
            "Authorization": "Bearer {env:BEEKEEPER_GATEWAY_TOKEN}"
          }
        }
      }
    }

Set the auth token in your shell environment before starting Trae:

  export BEEKEEPER_GATEWAY_TOKEN=$(beekeeper gateway token)

Get the current token:

  beekeeper gateway token

Coverage summary:
  - MCP tools:          INTERCEPTED (via MCP gateway)
  - Native shell/file tools: UNGUARDED (no programmatic pre-exec hook upstream)

`)
	return nil
}
