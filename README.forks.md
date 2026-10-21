# Alien21: minimal API fixes and upstream provenance

Branch: `develop-alien21`. Baseline: lejianwen/rustdesk-api v2.7,
`c5687e150615bfb00d9d105f0883a2802750a8ad`.

| Reference fork / commit | Adapted here |
| --- | --- |
| [Singular-Pharma/rustdesk-api](https://github.com/Singular-Pharma/rustdesk-api/commit/50b3923953339795c42e07f19c741f50e873bea8) (`singular`) | Administrator guard for all server-command routes, authenticated admin logout, missing command-update route. Adapted to this repository's existing layout. |
| [q1ngyang/rustdesk-api-kessoku](https://github.com/q1ngyang/rustdesk-api-kessoku/commit/fbcd9d0dbb2f0a5320bfcdf1c0e2e6cb0d64c746) (`master`; reviewed through `edfdd5b07f04bac6c3cdf96036fe9cb69164fb8d`) | Identity/owner conflict checks for device reports and login binding; address-book metadata lookup scoped to the requesting owner. |
| Local follow-up | The legacy address-book update path receives the same owner scope; heartbeat updates require the stored ID + UUID pair. Regression tests cover cross-account conflicts, unchanged legitimate native requests and opaque tokens. |

These are selective adaptations, not complete cherry-picks. We did not adopt a
new role system, monorepo layout, Starry device registry/proof, mandatory JWT or
new browser client. Existing optional upstream JWT code remains optional.
Invitations and hbbs/hbbr admission policy are not changed by these API patches.

## Native compatibility and security boundary

The supported native clients are original RustDesk 1.5.0 and Alien21 1.5.1.
Their sysinfo and heartbeat requests contain ID + UUID and do not carry a
Bearer token. Requiring authentication for every report would break them.

An unknown pair may first create an **unowned** inventory record. Later reports
must match the stored pair and cannot replace its ID, UUID or owner. Native
login may bind an unowned exact pair or rebind it to the same owner; another
account/identity cannot silently take it over. An existing owner-managed row
without a UUID can only receive that UUID through the same owner's native
login. Explicit logout keeps upstream unbinding behavior; it revokes its token
and releases that account's binding without discarding the stored pair.

This is first-use registration, not cryptographic device attestation. UUID is
not an authentication secret. A party that knows the exact pair can still
imitate anonymous reports; an attacker can pre-register an unused ID. Eliminating
these limitations needs an authenticated client/server proof and is outside
this compatible minimal patch. A shared API still has one global ID namespace:
independent hbbs installations with colliding IDs cannot both register that ID.
A conflict is rejected instead of overwriting someone else's device. API login
does not by itself authorize desktop control or prove private-server membership.

Metadata enrichment only reads inventory owned by the current account, both
for the newer peer-add endpoint and legacy address-book upload. It does not
change explicitly configured address-book sharing permissions.

## Checking the source forks again

Keep the pinned commits above as the review boundary. From this repository:

```sh
git remote add singular https://github.com/Singular-Pharma/rustdesk-api.git
git remote add kessoku https://github.com/q1ngyang/rustdesk-api-kessoku.git
git fetch singular singular
git fetch kessoku master
git log 50b3923953339795c42e07f19c741f50e873bea8..singular/singular --oneline
git log edfdd5b07f04bac6c3cdf96036fe9cb69164fb8d..kessoku/master --oneline
```

Add a remote only once. Review individual diffs and their native-wire behavior
before adapting a change; later commits may depend on incompatible server or
role-system changes. The original MIT license and source attributions remain.

Build paths are documented in [README.build.md](README.build.md). The separate
browser-client feasibility review is in [README.webclient.md](README.webclient.md).
