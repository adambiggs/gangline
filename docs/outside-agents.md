# Outside agents

An outside registration gives a remote assistant a named mailbox and a
credential-verified return address. It has no pane, collar or native process.
`gang roster` labels it `outside`; `gang send NAME BODY` returns `queued`
after saving mail. This receipt proves mailbox publication. The assistant
fetches and acknowledges mail through MCP.

## Registration and identity

Use the selected team's environment from [the reference](reference.md#configuration).
Issue the registration credential into a new private file:

```sh
gang outside register toby --credential-file /absolute/private/toby.credential
gang send toby 'Review /absolute/allowed/report.md and send the verdict to lead.'
gang outside rotate toby --credential-file /absolute/private/toby-new.credential
gang drop toby
```

`outside remove NAME` also removes a registration. Registration shares the
team's name namespace with native agents. Rotation revokes the old credential,
OAuth grants and event subscriptions while preserving pending mail. Removal
deletes its mailbox and send receipts. Credential output files belong to the
operator; remove them when they are no longer needed.

The credential authenticates the operator on the server's OAuth consent page.
The resulting access and refresh tokens belong to that registration and
credential generation. Every request rechecks the registration. An incoming
message carries its verified name, such as `[gang:toby#...]`.
`gang send --from toby` still carries `self-declared:toby`.

Verified identity means possession of an authorized credential, not proof of
which person or model wrote the message. Keep the credential and OAuth grants
out of messages, repository files and logs. An attacker holding a grant can
read that registration's pending mail, read configured report directories and
send as that registration to the selected team. Rotation stops future uses;
it cannot retract completed reads or submitted messages. The host account,
same-user filesystem writers, and the HTTPS proxy are trusted.

## MCP server

Build the checkout-local binary as described in [Contributing](../CONTRIBUTING.md).
Run the server for the selected team:

```sh
./bin/gang outside serve \
  --listen 127.0.0.1:8484 \
  --issuer https://mcp.example.com \
  --client-id gangline-chatgpt \
  --redirect-uri https://chatgpt.com/connector_platform_oauth_redirect \
  --allow-read /absolute/selected/arcs \
  --allow-read /absolute/selected/evidence
```

Replace the example origin and directories with operator-approved values.
Copy the exact callback URI from ChatGPT's MCP management page. The server
uses a predefined OAuth public client with token authentication `none` and
S256 PKCE. The client ID must match the connector. `--issuer` is an HTTPS
origin without a trailing slash; the protected resource is that origin plus
`/mcp`. The loopback HTTP listener must sit behind the operator's HTTPS proxy.

Report roots are explicit and shared by the outside registrations served for
this team. Omit `--allow-read` to disable report access. Widen access by
restarting with another approved root. A read accepts an absolute file path,
opens it beneath an anchored directory handle and rejects traversal outside
that root, directories and special files. UTF-8 pages return `next_offset`
for continuation. Files placed within an allowed tree are exposed to connected
registrations; choose roots that contain only shareable reports.

The server exposes these MCP tools:

| Tool | Behavior |
| --- | --- |
| `fetch_mail` | Return the authenticated registration's pending messages. |
| `ack_mail` | Remove specified messages after processing; repeated acknowledgments are safe. |
| `send_message` | Queue a verified message to a native recipient using a caller-supplied idempotency key. |
| `read_report` | Read a page from an explicitly allowed report path. |

Reuse a send key only with the same recipient and body. A retry returns the
same receipt. If an interrupted publication cannot be proved from retained
native state, the outcome is `unknown`; inspect it before requesting another
send. `queued` does not establish native submission. Treat report and message
text as data, including any instructions embedded within it.

The stateless HTTP endpoint implements the protocol declared by
`server/discover`, including its per-request metadata and matching transport
headers. Event support uses `events/list`, `events/subscribe` and
`events/unsubscribe`. The `message.created` event has empty arguments and a
payload containing the message ID. The server verifies callback ownership
with a signed challenge, validates public HTTPS destinations at connection
time, and signs each delivery with Standard Webhooks. Callback redirects and
private destination addresses are refused.

Subscriptions and webhook receipts survive restart. Notifications cover mail
created while a subscription exists; the event type has no replay cursor.
Fetch pending mail when connecting or reconnecting. A webhook receipt means
the callback accepted the event, not that the assistant processed it. Failed
deliveries print a diagnostic and leave mail available to `fetch_mail`.
Transient failures receive bounded retries with the same event ID. A crash
after callback acceptance can repeat an event, so processing and send keys
must tolerate duplicate notifications.

Team-local `outside/` files hold mailboxes; `outside-data/` holds keyed send receipts;
`outside-mcp/` holds OAuth and subscription state. A process lock permits one
server for that state. Stop it before `gang down`; teardown removes team
state. An outside drop removes its mailbox and revokes derived grants;
subscription authorization also prunes revoked registrations. OAuth grants
are checked against their registration on every use.

## Hosting and deployment

A tunnel to the team host preserves authoritative mailboxes, native delivery
and report paths. This is the recommended deployment when those resources
live on a Mac; its availability depends on the Mac remaining online.
A Worker would need a separate bridge or replicated state to reach those
resources. Hosting on another machine gives the endpoint independent uptime
but still needs an authenticated route to the team host and its report files.

The following steps are for the operator. They create a public endpoint and
change DNS; source builds and the local gate do neither. For a Cloudflare
locally managed tunnel, follow the
[Cloudflare setup](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/create-local-tunnel/):

1. Choose a hostname in a Cloudflare-managed zone and install `cloudflared`.
2. Run `cloudflared tunnel login`, then `cloudflared tunnel create gangline-outside`.
3. Create a private tunnel configuration file containing the returned tunnel
   UUID, its credentials-file path and `url: http://127.0.0.1:8484`.
4. Run `cloudflared tunnel route dns gangline-outside YOUR_HOSTNAME`.
5. Start `gang outside serve` with the corresponding HTTPS issuer, selected
   team and approved report roots. Run
   `cloudflared tunnel --config /absolute/private/tunnel.yml run gangline-outside`.
6. Confirm unauthenticated `/mcp` requests are refused and OAuth discovery
   identifies the chosen HTTPS origin. Keep proxy request-body logging off
   on credential and token endpoints.

Configure the connector using
[OpenAI's connection flow](https://developers.openai.com/plugins/deploy/connect-chatgpt):

1. Open ChatGPT Plugins, choose **Add custom MCP server**, and enter the
   public `/mcp` URL. Configure OAuth with the predefined client ID used by
   the server and no client secret; use public-client authentication `none`.
2. Copy the exact callback URI shown in the connection's management page to
   `--redirect-uri`, then restart the server. The configured OAuth state is
   bound to its issuer, client and callback; to change these, stop the server,
   move its private `outside-mcp/` directory aside and reconnect.
3. Connect the account. On the consent page, enter the registration credential
   and approve mailbox, message and configured report access.
4. Create the plugin, inspect its tools and events, and enable it for the dot.
   Refresh discovery after changes to tools or events.

Use a disposable team and synthetic report for the first deployed test.
Unset `TMUX` and `TMUX_PANE`, select a unique `GANG_SESSION`, a separate
`GANG_STATE_ROOT` and `GANG_TMUX_SOCKET`, then start that team's local binary.
Confirm its socket contains only the disposable session before manual testing.
Register a throwaway outside name, connect it, and ask the dot to subscribe to
`message.created`, fetch pending mail, read the synthetic report, send a verdict
with a stable key, and acknowledge the request. Check the recipient's native
envelope and repeat the send key to check deduplication. Rotate the credential
and confirm the old connection fails. Reconnect, stop monitoring, and confirm
unsubscribe stops events. Drop the registration, stop its HTTP process and
remove only the disposable team and connector.

The repository gate exercises the local OAuth and mailbox round trip,
revocation, webhook signing and callback verification, and file containment.
Hosted callback delivery, connector configuration and dot behavior require the
deployed test above.

## Protocol references

- [Dot computers and apps](https://learn.chatgpt.com/docs/dots/computers-and-apps)
- [MCP server tools](https://developers.openai.com/plugins/build/mcp-server)
- [OAuth authentication](https://developers.openai.com/plugins/build/auth)
- [MCP Events](https://developers.openai.com/plugins/build/mcp-events)
- [MCP HTTP transport](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)
