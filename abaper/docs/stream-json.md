# `abaper code --output stream-json`

`abaper code` (and `bai abap code`) run the ABAP agent headlessly. With
`-o stream-json`, stdout carries newline-delimited JSON — one event per line —
so an editor or script can follow the run without parsing human text.
Status messages and logs stay on stderr.

```sh
abaper code -o stream-json "create class ZCL_FOO with a method that adds two numbers"
```

## Contract

- Every stdout line is one JSON object with a `type` field.
- `done` is always the last event. Its `exit` matches the process exit code:
  `0` on success, `1` on any failure, including a failed verification.
- Unknown event types and unknown fields may be added in a minor release;
  clients should ignore what they don't recognise. Renaming or removing a
  field, or changing its meaning, is a breaking change.
- The agent events (`text`, `tool_use`, `tool_result`, `result`, `error`) use
  the same names and keys as `bai --print --output-format stream-json`.

## Events

In run order:

| `type` | Fields | When |
|---|---|---|
| `session` | `model`, `system` (active SAP system name, may be empty), `cwd` | Once, before the agent starts |
| `text` | `text` | Assistant text, streamed in chunks; concatenate them |
| `tool_use` | `id`, `name`, `input` (the arguments: an object, or a string if they weren't valid JSON) | The agent calls a tool |
| `tool_result` | `id`, `name`, `ok`, `output` | The tool finished; `ok` is false when the output is an error |
| `result` | `stop_reason` (`end_turn` or `max_turns`), `input_tokens` | One agent loop ended (again after each repair round) |
| `error` | `error` | The agent loop reported an error |
| `objects` | `items[]`: `type`, `name`, `ok`, `attempts`, `detail` (failure reason, only when `ok` is false) | Every SAP object the agent deployed, after the first loop |
| `verify` | `round` (0 first, then each repair round), `passed`, `report[]`: `type`, `name`, `syntax`, `active`, `tests`, `problems[]` | After each verification pass. Each step is `ok`, `failed`, `skipped`, or (tests only) `none` |
| `repair` | `round`, `of` | Failed objects go back to the agent for repair round `round` of `of` |
| `done` | `exit`, `error` (only when `exit` is 1) | Always last |

`objects` and `verify` are skipped when the agent deployed nothing; `verify` is
also skipped with `--no-verify`.

## Example

A run that deploys one class and passes verification:

```json
{"type":"session","model":"auto","system":"DEV","cwd":"/home/me/project"}
{"type":"tool_use","id":"t1","name":"abap_deploy","input":{"name":"ZCL_FOO","type":"class","source":"CLASS zcl_foo DEFINITION PUBLIC. …"}}
{"type":"tool_result","id":"t1","name":"abap_deploy","ok":true,"output":"Deployed and activated class ZCL_FOO."}
{"type":"text","text":"Deployed ZCL_FOO."}
{"type":"result","stop_reason":"end_turn","input_tokens":0}
{"type":"objects","items":[{"type":"class","name":"ZCL_FOO","ok":true,"attempts":1}]}
{"type":"verify","round":0,"passed":true,"report":[{"type":"class","name":"ZCL_FOO","syntax":"ok","active":"ok","tests":"ok"}]}
{"type":"done","exit":0}
```

The golden test `internal/cmd/testdata/abap_code_stream.golden` pins this
output; update it with `go test ./internal/cmd -run StreamJSONGolden -update`
only for an intended schema change.
