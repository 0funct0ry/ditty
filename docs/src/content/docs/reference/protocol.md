---
title: Protocol reference
description: The ditty.v1 wire protocol — opcode tables, the Hello payload, and a worked frame sequence, for anyone writing an alternative client.
---

All frames are **binary** WebSocket messages. Byte 0 is the opcode; bytes 1..n are the payload. Text frames are rejected with close code `1003`. The subprotocol string `ditty.v1` is required at upgrade time — a mismatch closes the connection with `1002`. Protocol changes bump to `ditty.v2`; nothing about the wire format changes silently within `ditty.v1`.

## Client → Server

| Op | Name | Payload | Notes |
|---|---|---|---|
| `0x00` | `Input` | raw bytes | Dropped silently if the Client is read-only. The server sends one `Notice` on the first drop only. |
| `0x01` | `Resize` | JSON `{"cols":int,"rows":int}` | Ignored unless this Client is the sizing client. Clamped to 10–1000 cols, 5–500 rows. |
| `0x02` | `Ping` | empty | Optional application-level keepalive; the server replies `Pong`. |

## Server → Client

| Op | Name | Payload |
|---|---|---|
| `0x00` | `Output` | raw PTY bytes |
| `0x01` | `Hello` | JSON — see below. Always the first frame after upgrade. |
| `0x02` | `Roster` | JSON `{"clients":[{"id","label","writable","joinedAt"}],"count":int}` |
| `0x03` | `State` | JSON `{"writable":bool,"sizing":bool,"state":"live\|detached\|closed","reason":string}` |
| `0x04` | `Exit` | JSON `{"code":int,"signal":string,"message":string}` |
| `0x05` | `Notice` | JSON `{"level":"info\|warn\|error","message":string}` |
| `0x06` | `Pong` | empty |

## `Hello`

The first frame the server ever sends, always:

```json
{
  "protocol": "ditty.v1",
  "server":   "1.0.0",
  "session":  { "id": "01JBQ8...", "name": "deploy", "title": "bash — web-01",
                "cols": 120, "rows": 34, "state": "live", "startedAt": "...",
                "shared": false },
  "client":   { "id": "01JBQ9...", "label": "Client 2", "writable": false, "sizing": false },
  "policy":   { "writable": false, "reconnect": true, "reconnectInterval": "3s",
                "unloadWarning": true, "maxClients": 0 },
  "profile":  { "theme": "ditty-dark", "fontFamily": "...", "...": "see the Themes page" }
}
```

## Sizing

The first attached Client with write capability is the sizing client; if none has write, the first-attached Client sizes. Others letterbox and see a one-shot `Notice`. When the sizing Client detaches, the role passes to the next Client by join order, announced by a `State` frame.

## Keepalive and deadlines

- The server sends a WebSocket `ping` every `--ping-interval` (default 25s). The read deadline is `ping-interval × 2 + 5s`, reset on any read.
- Every write carries a 5s write deadline — a wedged reader closes rather than blocking output fan-out to other Clients.
- Each Client has a 256-frame buffered send queue. Overflow closes that Client with code `1011`; a slow Client is dropped, never allowed to back-pressure the PTY for everyone else.
- Output is coalesced: PTY reads use a 32 KiB buffer with a 5ms debounce window (tunable via `--chunk-bytes`/`--flush-interval`), so a chatty process doesn't produce a flood of tiny frames.

## Worked frame sequence

A single Client connecting, typing, resizing, and the Command exiting — opcode byte shown first, then payload:

```
1. connect, upgrade succeeds (subprotocol ditty.v1 negotiated)

2. server → client   0x01 Hello    {"protocol":"ditty.v1","session":{...,"cols":80,"rows":24,"state":"live"},
                                      "client":{"id":"01J...","writable":true,"sizing":true}, ...}

3. server → client   0x00 Output   "$ "                       (raw bytes, the shell prompt)

4. client → server   0x00 Input    "ls\n"                     (raw bytes — this Client is writable)

5. server → client   0x00 Output   "ls\nfile1  file2\n$ "     (echo + command output, coalesced)

6. client → server   0x01 Resize   {"cols":120,"rows":40}     (this Client is the sizing client, so it applies)

7. server → client   0x03 State    {"writable":true,"sizing":true,"state":"live","reason":""}

8. (a second browser opens the same URL)
   server → client   0x02 Roster   {"clients":[{"id":"01J...","label":"Client 1","writable":true,...},
                                                 {"id":"01K...","label":"Client 2","writable":false,...}],
                                      "count":2}

9. client → server   0x00 Input    "exit\n"

10. server → client  0x04 Exit     {"code":0,"signal":"","message":"bash exited (code 0) after 4m12s"}
```

After step 10 the server closes the connection with a normal close code; there's no auto-reconnect on a clean `Exit` — only on an unexpected drop.

## Next

- [CLI reference](/reference/cli/run/) for every flag that shapes this protocol's behavior (`--ping-interval`, `--scrollback-bytes`, `--cols`/`--rows`, and so on).
- [Security](/security/) for what a read-only Client can and can't do at the protocol level.
