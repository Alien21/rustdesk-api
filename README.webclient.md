# Browser client: feasibility review (2026-10-22)

Reference frontend: [MrMarkus42/rustdesk-api-web, fedd7d4](https://github.com/MrMarkus42/rustdesk-api-web/tree/fedd7d48065a34ebbe51d0a3009817e0e5eb8c4b).
Companion API: [MrMarkus42/rustdesk-api, 7ce0913](https://github.com/MrMarkus42/rustdesk-api/tree/7ce09138fe280da881034bda06acca228077bc32).
This is a static source review; browser integration is not implemented here.

The frontend cannot simply be copied into this API. It needs authenticated
`GET /api/admin/webclient/config`, `POST /api/admin/webclient/ticket`,
`GET /api/admin/webclient/peer`, plus the `/webclient/ws/id` and `/relay` proxies.
The current static `/webclient/*filepath` route must be reconciled with that
WebSocket namespace to avoid a Gin wildcard collision.

The companion supports existing opaque API tokens without JWT. Its random,
single-use 30-second WS ticket admits a browser to a proxy connection; it is
different from our protected relay ticket and does not prove device ownership
or private membership.

Browser transport uses WebSocket hbbs/hbbr (21118/21119), always through relay.
The new native TCP KX greeting therefore does not itself enable this browser.
Its peer E2E implementation verifies the signed host key and supports our v1
key derivation. The existing fallback to picked version 0 must be removed:
this integration must require v1 and fail closed. API login must not bypass
host password, consent or 2FA.

| Server/access mode | Required work |
| --- | --- |
| Ordinary public server | API endpoints/proxy, HTTPS/WSS, strict v1, real 1.5.0/1.5.1 interoperability tests. |
| Private server with explicit public access, or a valid host-issued P- invitation | Existing WS public/guest relay adapter is usable in principle. Verify alias identity, guest relay, expiry and revocation end to end. |
| Invitation recipient without an API account | Add narrowly scoped guest bootstrap/admission tied to that invitation; the reference proxy requires API login. |
| Private member connecting to a host with public access disabled | Add a safe delegated grant/broker and relay proof. API account alone is not ACCESS_KEY membership; never expose shared ACCESS_KEY to the browser. |

Production work also includes Origin checks, target/session/usage bounds for
WS tickets, token revocation, frame/queue limits, and trusted or encrypted
API-to-hbbs/hbbr transport. Keep browser-to-host E2E keys out of the API/proxy.
Test wrong signatures and ciphertext, credentials/consent/2FA, expired/revoked
invitations, reconnects and disabled public access on actual Windows/Linux
hosts with Chromium/Edge WebCodecs.

Planning estimate for one experienced developer with test hosts available:

- First public demo: 2-4 working days.
- Production public access including guest invitations: approximately 5-8 days total.
- Full private membership: another 7-12 days, approximately 12-20 days total.

These are estimates, not tested compatibility claims, and do not include full
native-client feature parity. Existing native API routes need not change.

License provenance must also follow the imported files: the frontend's MIT
license does not replace the AGPL-3.0 provenance explicitly stated for its
RustDesk protobufs/generated protocol code in the reference README.
