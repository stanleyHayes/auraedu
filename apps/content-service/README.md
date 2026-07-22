# content-service

Owns AuraEDU Growth content policy, generated drafts, immutable versions, compliance findings,
and independent approval. It never auto-publishes generated work.

The production generator uses the OpenAI Responses API with `store: false`. Set
`OPENAI_API_KEY`, `CONTENT_AI_MODEL`, and optionally `OPENAI_BASE_URL`; production startup
fails closed without a provider key. Local runtime can start without a key, but generation
returns `503` until one is configured.

Every request is tenant-scoped, permission-checked (`content.generate` or `content.review`),
and gated by `growth_content_ai`. PostgreSQL FORCE RLS protects brand profiles, drafts,
versions, replay records, and the transactional outbox.
