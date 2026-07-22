# Production release evidence

`manifest.json` is the machine-checked record of evidence still required before AuraEDU can be called production ready. Its pending IDs must exactly match every unresolved status row in the live table in `agent_plan.md`; only an exact `Done` status is excluded. Custom wording such as `Implemented — proof pending` cannot bypass the gate.

## Recording evidence

1. Store a sanitized, immutable record below `release/evidence/records/<AURA-ID>/`. Never store credentials, access tokens, raw student data, provider secrets, or private message bodies.
2. Compute its SHA-256 digest with `shasum -a 256 <file>`.
3. Add an artifact object with `path` and lowercase `sha256` to the manifest item.
4. Set the top-level `release_git_sha` to the exact 40-character commit deployed as the release candidate. Every verified JSON record must carry that same `git_sha`; changing the candidate deliberately invalidates evidence from older deployments.
5. Once every stated requirement is proven, set the item to `verified`, add an RFC 3339 `verified_at` value at or after the evidence proof window and a named `approved_by` owner, and change the matching ledger row to `Done` in the same review.
6. Run `make release-evidence-validate`. The full production decision uses `make release-readiness`, which fails while any item or ledger row remains pending or the release commit is unset.

Screenshots alone are valid only for explicitly visual checks. Provider, load, isolation, recovery and deployment items require machine-readable results or exported provider records with secrets and personal data removed.

## Strict evidence profiles

Every manifest ID has a registered semantic validator. Adding a pending or verified item without a validator fails the release gate. A checksum proves that a file did not change; the profile proves that the file actually covers the release requirement.

Operational proofs (`AURA-8.1`, `AURA-18.10/18.11`, `AURA-47.3`, `AURA-48.7`, `AURA-48.8`, `AURA-59.2`, `AURA-9.1`, `AURA-9.3`, `AURA-9.4`, `AURA-9.5`, and `AURA-9.8`) require exactly one JSON record containing:

- the exact scenario name and staging or production environment;
- a credential-free HTTPS provider or deployment target;
- a bounded run ID, deployed Git SHA, and ordered proof window;
- every exact check registered for that story, each passing inside the proof window;
- a 16-character lowercase hexadecimal fingerprint for each provider observation.

Unknown fields are rejected so tokens, provider secrets, message contents, customer records, and unsanitized provider responses cannot be retained accidentally. The required check names live beside the validator in `tools/release/verify-readiness.mjs` and deliberately mirror each manifest requirement.

Visual proofs (`AURA-21.9`, `AURA-57.2`, `AURA-58.1`, `AURA-58.3`, and `AURA-59.2`) require exactly one JSON record plus the exact PNG set declared by that record. The validator requires every named state at every required desktop/mobile viewport, query-free application routes, unique screenshot files, manifest-backed hashes, genuine PNG framing, and image dimensions matching the declared viewport. An extra, missing, reused, malformed, or unreferenced screenshot fails the gate. Stories with both operational and visual profiles must supply one distinctly named JSON record for each profile; unregistered JSON profiles are rejected.

The performance, two-school isolation, and provider-delivery profiles remain scenario-specific and reject incomplete matrices, failed thresholds, placeholder provenance, and sensitive fields.

The AURA-50.2 staging isolation artifact has a stricter schema: it must be the immutable output from `tools/isolationtest`, prove at least ten resource domains in both school directions, and retain exactly the positive-control `200`, cross-resource `404`, and token/header-mismatch `403` result for every probe. The release verifier rejects raw tenant codes, tokens, resource IDs, response bodies, incomplete matrices, duplicate checks, placeholder targets, and failed outcomes.

The AURA-18.9 email-provider artifact must be the immutable output from `tools/providerprobe`. It proves the deployed API created a pending message, Resend accepted its envelope, a separate read-back observed the persisted `sent` result, and the signed webhook path later projected `delivery_status=delivered`. The verifier requires all four exact outcomes and rejects raw tenant, recipient, message, mailbox, token, content or provider-response fields.

The AURA-18.10/18.11 Twilio artifact must be the immutable output from
`tools/twilioprobe` and prove both channels independently:
SMS provider acceptance, SMS delivery and persisted SMS status, followed by
WhatsApp provider acceptance, WhatsApp delivery and persisted WhatsApp status.
The deployed callback URL must match `TWILIO_STATUS_CALLBACK_URL` exactly so
Twilio's signature remains verifiable through the Vercel relay. Evidence may
retain only the public target, release provenance, timing and bounded
fingerprints; phone numbers, account/sender IDs, auth tokens, message bodies,
provider SIDs and callback bodies are forbidden.

```sh
cd tools/twilioprobe
GOWORK=off go run . \
  -config ../../release/scenarios/staging-twilio-providers.json \
  -out ../../release/evidence/records/AURA-18.10-18.11/twilio.json
```

The AURA-9.8 frontend-deployment artifact must be the immutable output from `tools/vercelprobe`. It binds two distinct linked Next.js projects and their `READY` production deployments to the release Git SHA, confirms required production-scoped environment keys without decrypting their values, checks each public app's identity and security headers, and observes exact API Gateway preflights from both frontend origins. The evidence retains only the public portal target and sanitized fingerprints; Vercel tokens, team/project IDs, environment values and API payloads are excluded.
