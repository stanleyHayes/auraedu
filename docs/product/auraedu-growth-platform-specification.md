# AuraEDU Growth Platform
## Product and Engineering Specification for AI-Assisted Student Recruitment, Marketing, Admissions, and Institutional Growth

**Document status:** Implementation blueprint
**Intended users:** Codex, Claude Code, Gemini, Kimi, human engineers, architects, product managers, and QA teams
**Parent product:** AuraEDU
**Recommended repository strategy:** One AuraEDU monorepo with independently deployable services
**Primary market:** Basic schools, senior high schools, tertiary institutions, and especially private universities
**Initial geographic focus:** Ghana, followed by West Africa and other African markets

---

# 1. Executive Summary

AuraEDU Growth is an extension of the existing AuraEDU education platform. It transforms AuraEDU from a school management system into an AI-powered education operating system that helps institutions:

1. attract prospective students;
2. capture and qualify leads;
3. answer enquiries continuously;
4. run multichannel marketing campaigns;
5. guide applicants through admissions;
6. convert admitted applicants into enrolled students;
7. analyse marketing and admissions performance;
8. improve future campaigns using approved feedback loops;
9. retain students and strengthen alumni engagement.

The system must not operate as an uncontrolled autonomous agent. It should automate routine work, recommend actions, and execute low-risk actions under institution-defined policies. High-impact actions such as spending advertising money, changing fees, making admission decisions, publishing sensitive announcements, or contacting large audiences must require human approval unless explicitly configured otherwise.

AuraEDU Growth should share AuraEDU's identity, tenant management, feature flags, billing, audit logging, notification infrastructure, and data governance controls. It should remain modular so institutions can enable only the capabilities they need.

---

# 2. Repository Decision

## 2.1 Recommendation

Do **not** create a completely separate product repository at this stage.

Add AuraEDU Growth to the existing AuraEDU product as a new set of bounded contexts and independently deployable services.

Use a monorepo initially.

Recommended repository name:

```text
auraedu
```

Recommended high-level layout:

```text
auraedu/
├── apps/
│   ├── admin-web/
│   ├── institution-web/
│   ├── applicant-portal/
│   ├── student-portal/
│   ├── parent-portal/
│   ├── marketing-site/
│   └── mobile/
├── services/
│   ├── identity-service/
│   ├── tenant-service/
│   ├── feature-service/
│   ├── admissions-service/
│   ├── crm-service/
│   ├── campaign-service/
│   ├── content-service/
│   ├── communication-service/
│   ├── ai-orchestrator-service/
│   ├── knowledge-service/
│   ├── feedback-service/
│   ├── analytics-service/
│   ├── reputation-service/
│   ├── billing-service/
│   ├── notification-service/
│   ├── audit-service/
│   └── integration-service/
├── packages/
│   ├── ui/
│   ├── contracts/
│   ├── config/
│   ├── observability/
│   ├── auth/
│   ├── events/
│   ├── testing/
│   └── sdk/
├── infrastructure/
│   ├── docker/
│   ├── kubernetes/
│   ├── terraform/
│   ├── monitoring/
│   └── local/
├── docs/
│   ├── architecture/
│   ├── product/
│   ├── api/
│   ├── adr/
│   ├── security/
│   └── runbooks/
├── scripts/
├── .github/
│   └── workflows/
├── go.work
├── pnpm-workspace.yaml
├── package.json
├── Makefile
└── README.md
```

## 2.2 Why the same repository is better now

AuraEDU Growth depends on existing shared platform capabilities:

- institution tenancy;
- users and permissions;
- student and applicant identity;
- school branding;
- feature flags;
- subscriptions;
- notifications;
- audit logs;
- application and admission data;
- school programme and fee data;
- analytics;
- AI infrastructure.

Creating a separate repository and separate platform now would duplicate these capabilities and increase:

- authentication complexity;
- deployment complexity;
- integration failures;
- duplicated schemas;
- inconsistent permissions;
- operational cost;
- engineering overhead.

## 2.3 When separate repositories may become appropriate

Split a service into its own repository only when one or more of these conditions become true:

- the service has an independent engineering team;
- it requires a different release cadence;
- it has strict security or regulatory isolation requirements;
- it is sold as a standalone external product;
- it has significantly different infrastructure needs;
- monorepo build and deployment times become difficult to manage;
- ownership boundaries are stable and well understood.

The preferred evolution is:

```text
Modular monolith or small service set
        ↓
Well-defined bounded contexts
        ↓
Independently deployable services
        ↓
Separate repositories only where justified
```

Do not start with dozens of distributed services merely because the target architecture is described as microservices.

---

# 3. Product Vision

## 3.1 Vision statement

AuraEDU is the AI operating system for education institutions.

It helps institutions manage operations, grow enrolment, improve student outcomes, and make better decisions through secure automation and institution-controlled intelligence.

## 3.2 AuraEDU product pillars

### AuraEDU Core

Runs institutional operations:

- student information;
- attendance;
- academics;
- assessments;
- finance;
- HR;
- timetables;
- parent communication;
- student portals;
- learning support.

### AuraEDU Growth

Grows the institution:

- lead generation;
- marketing automation;
- admissions CRM;
- applicant nurturing;
- campaign management;
- social media operations;
- content generation;
- website chatbot;
- WhatsApp assistant;
- email and SMS journeys;
- enrolment conversion;
- reputation monitoring.

### AuraEDU Intelligence

Explains and predicts:

- admission performance;
- campaign effectiveness;
- enrolment forecasts;
- programme demand;
- dropout risk;
- financial projections;
- student sentiment;
- competitor activity;
- recommended interventions.

---

# 4. Business Objectives

The platform should help institutions achieve measurable outcomes.

Primary objectives:

- increase qualified leads;
- reduce response time to enquiries;
- increase application completion rate;
- increase admission acceptance rate;
- increase admitted-to-enrolled conversion;
- reduce cost per enrolled student;
- identify high-performing programmes and channels;
- reduce applicant abandonment;
- improve campaign execution speed;
- provide management with real-time performance visibility.

Example key metrics:

```text
Website visitor → lead conversion
Lead → application conversion
Application started → application completed
Application completed → admitted
Admitted → offer accepted
Offer accepted → deposit paid
Deposit paid → fully enrolled
Cost per lead
Cost per completed application
Cost per enrolled student
Campaign return on investment
Average first-response time
Average time from enquiry to enrolment
Applicant satisfaction score
AI answer accuracy
Human escalation rate
```

---

# 5. Users and Roles

## 5.1 Platform roles

- AuraEDU platform administrator
- tenant administrator
- institution owner
- vice chancellor or principal
- registrar
- admissions director
- admissions officer
- marketing director
- marketing officer
- finance officer
- programme coordinator
- content reviewer
- customer support officer
- data analyst
- compliance officer
- applicant
- parent or guardian
- student
- alumnus
- external agency user
- AI service account
- integration service account

## 5.2 Permission model

Use role-based access control combined with tenant isolation and optional attribute-based restrictions.

Example permissions:

```text
crm.lead.read
crm.lead.create
crm.lead.assign
crm.lead.export
campaign.read
campaign.create
campaign.approve
campaign.publish
campaign.budget.approve
content.generate
content.review
content.publish
admissions.application.read
admissions.application.review
admissions.offer.issue
analytics.executive.read
ai.agent.configure
ai.action.approve
knowledge.manage
feedback.review
audit.read
integration.manage
```

Every API request must carry:

- authenticated subject;
- tenant identifier;
- institution identifier where applicable;
- role and permissions;
- request correlation identifier.

---

# 6. Multi-Tenancy

AuraEDU Growth must support multiple institutions from one platform.

Each tenant must have:

- isolated data;
- institution branding;
- institution domains;
- configurable feature flags;
- independent programmes and fees;
- independent knowledge base;
- independent communication templates;
- independent AI configuration;
- independent approval policies;
- independent integrations;
- independent usage limits;
- independent billing plan.

Recommended tenant hierarchy:

```text
Platform
└── Tenant
    ├── Institution
    │   ├── Campus
    │   ├── Faculty
    │   ├── Department
    │   └── Programme
    └── Institution
```

A tenant may contain one institution or a group of related institutions.

---

# 7. Feature Flag Strategy

Features must be independently enabled or disabled per tenant.

Example flags:

```text
growth.crm.enabled
growth.campaigns.enabled
growth.content_ai.enabled
growth.website_chat.enabled
growth.whatsapp.enabled
growth.sms.enabled
growth.voice.enabled
growth.reputation_monitor.enabled
growth.competitor_monitor.enabled
growth.predictive_analytics.enabled
growth.autonomous_actions.enabled
growth.application_assistant.enabled
growth.lead_scoring.enabled
growth.alumni_marketing.enabled
```

Feature evaluation should support:

- global default;
- subscription plan;
- tenant override;
- institution override;
- percentage rollout;
- environment restriction;
- temporary emergency disable.

---

# 8. Core User Journeys

## 8.1 Prospect discovery journey

```text
Prospect sees campaign
→ visits landing page
→ asks AI assistant a question
→ receives programme recommendations
→ submits contact details
→ becomes a lead
→ receives a personalised follow-up
→ starts application
→ completes application
→ receives admission outcome
→ accepts offer
→ pays deposit
→ becomes enrolled student
```

## 8.2 Marketing officer journey

```text
Marketing officer defines campaign objective
→ selects audience and programmes
→ AI proposes campaign strategy
→ AI generates content variants
→ reviewer edits and approves content
→ campaign is scheduled
→ content is published across channels
→ leads are attributed to campaign
→ dashboard reports performance
→ AI recommends improvements
```

## 8.3 Admissions officer journey

```text
New lead enters CRM
→ lead is enriched and scored
→ assigned to admissions officer
→ automated nurturing begins
→ officer sees lead history
→ applicant starts application
→ missing documents are detected
→ reminders are sent
→ completed application moves to review
→ offer is issued
→ conversion is tracked
```

## 8.4 Executive journey

```text
Executive opens dashboard
→ sees current funnel and revenue forecast
→ asks natural-language question
→ receives cited answer from institutional data
→ reviews anomalies and recommendations
→ approves high-impact action
```

---

# 9. Functional Modules

## 9.1 Admissions CRM

Capabilities:

- lead capture;
- lead import;
- lead deduplication;
- lead assignment;
- lead ownership;
- lead segmentation;
- lead scoring;
- lifecycle stages;
- interaction history;
- notes and tasks;
- consent tracking;
- follow-up scheduling;
- source attribution;
- campaign attribution;
- applicant conversion;
- custom fields;
- bulk operations;
- pipeline dashboards.

Recommended lead stages:

```text
new
contacted
engaged
qualified
application_started
application_completed
under_review
admitted
offer_accepted
deposit_paid
enrolled
lost
deferred
withdrawn
```

## 9.2 Campaign Management

Capabilities:

- create campaign;
- define objective;
- select audience;
- select programme;
- define channel;
- set dates;
- set budget;
- approval workflow;
- generate tracking parameters;
- schedule content;
- collect performance data;
- calculate conversion and ROI;
- compare variants;
- pause or terminate campaigns.

Supported channels:

- website;
- email;
- SMS;
- WhatsApp;
- Facebook;
- Instagram;
- TikTok;
- YouTube;
- LinkedIn;
- radio script preparation;
- event and open-day campaigns;
- referral programmes;
- school visits;
- agent and affiliate channels.

## 9.3 AI Content Studio

Content types:

- social posts;
- captions;
- ad copy;
- short video scripts;
- landing-page copy;
- email campaigns;
- SMS messages;
- WhatsApp sequences;
- programme descriptions;
- brochures;
- prospectus drafts;
- event invitations;
- scholarship announcements;
- radio scripts;
- FAQs;
- applicant guides.

Controls:

- institution tone of voice;
- approved terminology;
- prohibited claims;
- programme facts;
- fee accuracy;
- required disclaimers;
- content approval;
- version history;
- content expiry date;
- brand compliance.

Generated content must not be published automatically by default.

## 9.4 Website AI Assistant

Capabilities:

- answer programme questions;
- answer fee questions;
- explain entry requirements;
- recommend programmes;
- capture leads;
- start applications;
- retrieve application status;
- schedule calls;
- escalate to humans;
- support multiple languages;
- cite the institution source used;
- display uncertainty;
- refuse unsupported claims.

The assistant must use retrieval-augmented generation over tenant-approved content.

## 9.5 WhatsApp Admissions Assistant

Capabilities:

- answer FAQs;
- capture prospect data;
- share programme links;
- recommend programmes;
- send application reminders;
- collect non-sensitive preliminary information;
- provide application status;
- route to admissions officers;
- log interactions in CRM.

Do not collect highly sensitive identity documents directly through conversational channels unless secure upload workflows are implemented.

## 9.6 Email and SMS Journeys

Examples:

- new enquiry response;
- brochure follow-up;
- abandoned application;
- missing document reminder;
- application completion confirmation;
- offer notification;
- offer acceptance reminder;
- deposit reminder;
- orientation reminder;
- open-day invitation;
- scholarship deadline;
- alumni referral campaign.

Each journey must support:

- triggers;
- delays;
- conditions;
- branches;
- templates;
- personalisation;
- consent checks;
- quiet hours;
- frequency limits;
- cancellation;
- analytics.

## 9.7 Lead Scoring

Initial model should be rules-based, not machine learning.

Example score inputs:

- programme fit;
- entry qualification fit;
- engagement frequency;
- pages visited;
- brochure downloaded;
- event attendance;
- application started;
- application completion percentage;
- communication response;
- location;
- preferred intake;
- affordability indicators supplied voluntarily;
- previous deferral;
- referral source.

Do not use protected or sensitive personal characteristics for unfair discrimination.

Later versions may add supervised models after sufficient quality data exists.

Every score must expose:

- score;
- confidence;
- top positive factors;
- top negative factors;
- model or rule version;
- timestamp.

## 9.8 Programme Recommendation Engine

Inputs:

- academic background;
- subjects;
- grades;
- interests;
- career goals;
- location preference;
- study mode;
- budget range;
- preferred intake.

Outputs:

- ranked programme recommendations;
- eligibility status;
- missing requirements;
- estimated fees;
- related career paths;
- links to verified programme information.

Recommendations must clearly distinguish:

- eligible;
- potentially eligible;
- not currently eligible;
- insufficient information.

## 9.9 Application Completion Assistant

Capabilities:

- application checklist;
- completion percentage;
- missing fields;
- missing documents;
- invalid document detection;
- deadline reminders;
- contextual help;
- secure upload;
- payment guidance;
- human escalation.

## 9.10 Reputation and Social Listening

Capabilities:

- ingest public mentions from supported sources;
- categorise sentiment;
- detect recurring issues;
- identify misinformation;
- alert institution staff;
- link issues to programmes or campuses;
- generate response drafts;
- track resolution.

No automated public response should be published without approval by default.

## 9.11 Competitor Intelligence

Capabilities:

- manually maintain competitor list;
- monitor publicly available programme pages;
- record fees, scholarships, deadlines, and campaigns;
- compare programme offerings;
- identify market changes;
- generate periodic summaries.

The system must respect website terms, robots directives, rate limits, copyright, and applicable laws. It should prefer official public APIs and manual data collection where automated collection is not permitted.

## 9.12 Open Day and Event Management

Capabilities:

- event landing page;
- registration;
- QR check-in;
- attendee segmentation;
- reminder campaigns;
- programme interest capture;
- staff assignment;
- lead creation;
- post-event follow-up;
- feedback survey;
- conversion attribution.

## 9.13 Executive Intelligence

Natural-language questions may include:

- Why did applications fall this month?
- Which campaign generated the most enrolled students?
- Which programmes have low conversion?
- What is our expected enrolment this intake?
- Which leads need urgent follow-up?
- Which regions produce the highest-value applicants?

Every answer must include:

- source datasets;
- time range;
- filters;
- calculation notes;
- confidence;
- links to underlying dashboard data.

---

# 10. AI Agent Architecture

## 10.1 Principle

Use specialised agents coordinated by an orchestrator. Agents should not have unrestricted system access.

Recommended agents:

- Growth Strategist Agent
- Campaign Agent
- Content Agent
- Admissions Agent
- Applicant Support Agent
- Analytics Agent
- Reputation Agent
- Knowledge Curator Agent
- Feedback Analyst Agent
- Executive Assistant Agent

## 10.2 Agent execution model

```text
User or event
→ AI Orchestrator
→ policy check
→ context assembly
→ agent selection
→ tool permission check
→ agent execution
→ output validation
→ approval check
→ action execution
→ audit log
→ feedback capture
```

## 10.3 Agent permissions

Each agent must have an explicit allowlist.

Example:

```yaml
agent: campaign-agent
allowed_tools:
  - campaign.read
  - analytics.read
  - content.generate
  - audience.estimate
prohibited_tools:
  - admission.offer.issue
  - billing.change_plan
  - campaign.spend_without_approval
requires_approval:
  - campaign.publish
  - campaign.activate_paid_ads
```

## 10.4 Approval levels

### Level 0: Read-only

- retrieve information;
- summarise;
- analyse;
- recommend.

### Level 1: Draft

- create draft content;
- prepare campaign;
- prepare email;
- create task.

### Level 2: Low-risk execution

- send an individual reminder;
- update lead status;
- schedule an approved post;
- assign a lead based on rules.

### Level 3: Approval required

- send bulk communication;
- publish public content;
- modify a campaign;
- spend advertising budget;
- change programme information;
- issue admission offers.

### Level 4: Prohibited for AI

- final admission decision without approved institutional rule;
- altering grades;
- changing official fees without authority;
- deleting audit history;
- bypassing consent;
- accessing another tenant;
- changing security roles autonomously.

---

# 11. Feedback Loop and Continuous Improvement

## 11.1 Correct interpretation of a feedback loop

The system should not blindly retrain itself from every interaction.

The feedback loop should improve:

- knowledge coverage;
- retrieval ranking;
- prompts;
- workflows;
- content templates;
- routing;
- rules;
- model selection;
- escalation policies.

## 11.2 Feedback sources

- thumbs up or down;
- applicant satisfaction;
- officer correction;
- rejected generated content;
- edited generated content;
- unresolved questions;
- escalation reason;
- campaign conversion;
- application abandonment;
- incorrect answer report;
- outdated information report.

## 11.3 Feedback pipeline

```text
Interaction occurs
→ result is captured
→ user or staff provides feedback
→ feedback is classified
→ possible root cause is identified
→ proposed improvement is generated
→ human reviewer approves change
→ change is versioned
→ controlled evaluation is performed
→ new version is released
→ impact is monitored
```

## 11.4 Improvement types

```text
knowledge_gap
outdated_information
retrieval_failure
hallucination
wrong_workflow
wrong_audience
bad_tone
policy_violation
low_conversion
human_escalation_needed
integration_failure
```

## 11.5 Evaluation sets

Maintain tenant-specific and platform-level test sets.

Test categories:

- programme requirements;
- fees;
- deadlines;
- scholarships;
- campus information;
- application status;
- ambiguous questions;
- unsafe requests;
- unsupported claims;
- multilingual questions;
- adversarial prompts;
- tenant isolation.

No prompt, knowledge, or workflow update should be promoted without passing evaluation thresholds.

---

# 12. Knowledge Architecture

## 12.1 Knowledge sources

- programme catalogue;
- admission requirements;
- fees;
- scholarships;
- academic calendar;
- application deadlines;
- policies;
- campus information;
- accommodation;
- FAQs;
- official announcements;
- approved marketing content;
- support procedures.

## 12.2 Knowledge ingestion

```text
Source uploaded or connected
→ text extracted
→ metadata added
→ content reviewed
→ chunks generated
→ embeddings created
→ tenant index updated
→ evaluation tests run
→ version activated
```

Required metadata:

- tenant ID;
- institution ID;
- source type;
- title;
- owner;
- effective date;
- expiry date;
- approval status;
- programme;
- campus;
- intake;
- confidentiality level;
- version.

## 12.3 Retrieval requirements

- strict tenant filtering;
- institution filtering;
- source citation;
- freshness ranking;
- approved-content filtering;
- effective-date filtering;
- access-level filtering;
- result confidence;
- fallback to human support.

---

# 13. Suggested Service Boundaries

## 13.1 Identity Service

Responsibilities:

- authentication;
- session management;
- service accounts;
- multi-factor authentication;
- token issuance;
- password reset;
- external identity providers.

## 13.2 Tenant Service

Responsibilities:

- tenants;
- institutions;
- campuses;
- branding;
- tenant configuration;
- data residency settings.

## 13.3 Feature Service

Responsibilities:

- feature flags;
- plan entitlements;
- rollout rules;
- emergency disables.

## 13.4 CRM Service

Responsibilities:

- leads;
- contacts;
- lifecycle stages;
- assignments;
- activities;
- tasks;
- consent;
- segmentation.

## 13.5 Admissions Service

Responsibilities:

- applications;
- documents;
- reviews;
- decisions;
- offers;
- acceptance;
- admission rules;
- intake management.

## 13.6 Campaign Service

Responsibilities:

- campaigns;
- audiences;
- budgets;
- schedules;
- variants;
- attribution;
- campaign states;
- approvals.

## 13.7 Content Service

Responsibilities:

- generated content;
- templates;
- brand rules;
- reviews;
- approvals;
- publishing state;
- versioning.

## 13.8 Communication Service

Responsibilities:

- email;
- SMS;
- WhatsApp;
- web chat;
- future voice;
- template rendering;
- delivery status;
- inbound message routing;
- provider adapters.

## 13.9 AI Orchestrator Service

Responsibilities:

- model routing;
- prompt assembly;
- tool execution;
- agent policies;
- approval workflows;
- output validation;
- token and cost tracking;
- conversation state.

## 13.10 Knowledge Service

Responsibilities:

- document ingestion;
- chunking;
- embeddings;
- retrieval;
- citations;
- document approval;
- versioning.

## 13.11 Feedback Service

Responsibilities:

- feedback events;
- classification;
- improvement proposals;
- evaluation sets;
- release gates.

## 13.12 Analytics Service

Responsibilities:

- metrics;
- funnel calculations;
- attribution;
- forecasts;
- dashboards;
- executive queries;
- warehouse exports.

## 13.13 Integration Service

Responsibilities:

- social platforms;
- payment providers;
- advertising platforms;
- webhooks;
- external SIS;
- learning platforms;
- government or accreditation systems where permitted.

## 13.14 Audit Service

Responsibilities:

- immutable activity logs;
- AI decision logs;
- approval logs;
- security events;
- compliance exports.

---

# 14. Communication and Events

Use synchronous APIs for immediate requests and asynchronous events for cross-service workflows.

Recommended event transport:

- NATS JetStream for a simpler start; or
- Kafka when scale and event retention requirements justify it.

Example domain events:

```text
lead.created
lead.updated
lead.assigned
lead.qualified
application.started
application.completed
application.abandoned
application.admitted
offer.accepted
deposit.paid
student.enrolled
campaign.created
campaign.approved
campaign.published
campaign.performance.updated
content.generated
content.approved
message.sent
message.delivered
message.failed
message.replied
ai.answer.generated
ai.answer.escalated
feedback.submitted
knowledge.updated
```

Every event should include:

```json
{
  "event_id": "uuid",
  "event_type": "lead.created",
  "event_version": 1,
  "occurred_at": "RFC3339 timestamp",
  "tenant_id": "uuid",
  "institution_id": "uuid",
  "actor_id": "uuid",
  "correlation_id": "uuid",
  "causation_id": "uuid",
  "payload": {}
}
```

---

# 15. Data Storage

Recommended starting choices:

- PostgreSQL for transactional data;
- MongoDB only where flexible content structures provide clear value;
- Redis for caching, rate limits, locks, and short-lived state;
- object storage for documents and media;
- pgvector or a dedicated vector database for embeddings;
- ClickHouse, BigQuery, or PostgreSQL initially for analytics depending on scale.

Do not introduce both MongoDB and PostgreSQL into every service.

Each service owns its data. Other services access it through APIs or events.

---

# 16. Core Data Models

## 16.1 Lead

```text
id
tenant_id
institution_id
first_name
last_name
email
phone
country
region
city
preferred_programme_ids
preferred_intake_id
source
campaign_id
stage
score
score_version
owner_user_id
consent_email
consent_sms
consent_whatsapp
consent_voice
created_at
updated_at
```

## 16.2 Campaign

```text
id
tenant_id
institution_id
name
objective
status
channel
audience_definition
programme_ids
budget
currency
start_at
end_at
approval_status
owner_user_id
created_at
updated_at
```

## 16.3 Interaction

```text
id
tenant_id
lead_id
channel
direction
actor_type
actor_id
content_reference
summary
sentiment
occurred_at
```

## 16.4 AI Run

```text
id
tenant_id
agent_name
agent_version
model_provider
model_name
prompt_version
input_reference
output_reference
tool_calls
approval_required
approval_status
cost
latency_ms
status
created_at
```

## 16.5 Feedback

```text
id
tenant_id
interaction_id
ai_run_id
feedback_type
rating
comment
classification
review_status
reviewed_by
created_at
```

---

# 17. API Design

Use REST for most external and administrative APIs. Use gRPC where internal service-to-service communication benefits from strict contracts and high throughput.

Version public APIs:

```text
/api/v1/...
```

Example endpoints:

```text
POST   /api/v1/leads
GET    /api/v1/leads
GET    /api/v1/leads/{id}
PATCH  /api/v1/leads/{id}
POST   /api/v1/leads/{id}/assign
POST   /api/v1/leads/{id}/qualify

POST   /api/v1/campaigns
GET    /api/v1/campaigns
GET    /api/v1/campaigns/{id}
POST   /api/v1/campaigns/{id}/submit-for-approval
POST   /api/v1/campaigns/{id}/approve
POST   /api/v1/campaigns/{id}/publish

POST   /api/v1/content/generate
POST   /api/v1/content/{id}/approve
POST   /api/v1/content/{id}/publish

POST   /api/v1/ai/chat
POST   /api/v1/ai/recommend-programmes
POST   /api/v1/ai/executive-query

POST   /api/v1/feedback
GET    /api/v1/analytics/funnel
GET    /api/v1/analytics/campaigns
GET    /api/v1/analytics/enrolment-forecast
```

All list endpoints should support:

- pagination;
- filtering;
- sorting;
- field selection;
- tenant-safe search.

---

# 18. Frontend Applications

## 18.1 Institution Admin

Primary modules:

- overview dashboard;
- CRM;
- campaigns;
- content studio;
- communications;
- admissions;
- AI assistants;
- knowledge base;
- analytics;
- approvals;
- integrations;
- settings.

## 18.2 Applicant Portal

Features:

- programme discovery;
- eligibility check;
- saved programmes;
- application;
- document upload;
- payment;
- status tracking;
- messages;
- offer acceptance;
- onboarding.

## 18.3 Public Marketing Site

Features:

- tenant-branded landing pages;
- programme pages;
- SEO;
- campaign pages;
- forms;
- AI assistant;
- event registration;
- analytics consent;
- accessibility.

## 18.4 Mobile

Initial recommendation:

Build a responsive web application first. Add React Native or another mobile client only after mobile-specific needs are validated.

---

# 19. Non-Functional Requirements

## 19.1 Security

- tenant isolation at every layer;
- encryption in transit and at rest;
- least-privilege permissions;
- secure secret management;
- multi-factor authentication for privileged roles;
- immutable audit logs;
- signed webhooks;
- rate limiting;
- IP and device anomaly detection;
- input validation;
- output encoding;
- malware scanning for uploads;
- secure document access;
- dependency and container scanning.

## 19.2 Privacy

- collect only necessary personal data;
- consent records;
- purpose limitation;
- retention policies;
- data export;
- deletion workflows where legally permitted;
- masking of sensitive data;
- configurable data residency;
- privacy notices;
- opt-out and suppression lists.

## 19.3 AI Safety

- source-grounded responses;
- no fabricated programme or fee information;
- no cross-tenant data leakage;
- explicit uncertainty;
- human escalation;
- prompt injection defence;
- tool allowlists;
- output schema validation;
- cost and rate controls;
- action approvals;
- model and prompt versioning.

## 19.4 Reliability

Initial targets:

```text
99.9% monthly availability for core APIs
99.5% monthly availability for non-critical AI functions
RPO: 15 minutes
RTO: 2 hours
```

## 19.5 Performance

Example targets:

```text
P95 API response under 500 ms for standard CRUD
P95 dashboard query under 2 seconds
First chatbot response under 4 seconds
Message ingestion acknowledgement under 1 second
```

## 19.6 Accessibility

Target WCAG 2.2 AA for web applications.

---

# 20. Observability

Use OpenTelemetry.

Capture:

- distributed traces;
- structured logs;
- metrics;
- audit events;
- AI token usage;
- AI cost;
- retrieval quality;
- tool failures;
- queue lag;
- delivery rates;
- conversion events.

Required dashboards:

- service health;
- tenant usage;
- communication delivery;
- AI performance;
- campaign pipeline;
- admissions funnel;
- security events;
- cost by tenant;
- provider failures.

---

# 21. Integration Strategy

Use adapter interfaces to avoid vendor lock-in.

Potential categories:

- email providers;
- SMS providers;
- WhatsApp Business providers;
- payment gateways;
- social media platforms;
- advertising platforms;
- analytics providers;
- cloud storage;
- identity providers;
- calendar providers;
- video generation providers;
- CRM imports;
- existing school information systems.

Example Go interface:

```go
type MessageProvider interface {
    Send(ctx context.Context, message Message) (DeliveryReceipt, error)
    Status(ctx context.Context, providerMessageID string) (DeliveryStatus, error)
}
```

---

# 22. Deployment Architecture

Initial production architecture:

```text
CDN / WAF
    ↓
API Gateway
    ↓
Web applications and service APIs
    ↓
PostgreSQL / Redis / Object Storage / Vector Store
    ↓
Event Bus
    ↓
Workers and integrations
```

Recommended deployment stages:

### Stage 1

- Docker Compose for local development;
- managed PostgreSQL;
- managed Redis;
- object storage;
- one or a few deployable backend services;
- managed container platform.

### Stage 2

- Kubernetes only when operational need is proven;
- horizontal autoscaling;
- separate workers;
- event-driven integrations;
- data warehouse.

Do not adopt Kubernetes solely for appearance or future possibility.

---

# 23. Development Standards

## 23.1 Backend

Preferred:

- Go;
- hexagonal architecture;
- domain-driven boundaries;
- dependency injection through constructors;
- explicit interfaces;
- structured errors;
- context propagation;
- idempotency;
- database migrations;
- contract tests.

Example service layout:

```text
internal/
├── domain/
├── application/
├── ports/
├── adapters/
│   ├── http/
│   ├── postgres/
│   ├── events/
│   └── providers/
└── platform/
```

## 23.2 Frontend

Preferred:

- React;
- TypeScript;
- MUI;
- TanStack Query;
- React Hook Form;
- Zod;
- accessible components;
- shared design system;
- tenant themes.

## 23.3 Testing

Required levels:

- unit tests;
- integration tests;
- API contract tests;
- repository tests;
- end-to-end tests;
- tenant isolation tests;
- security tests;
- AI evaluation tests;
- load tests for critical paths.

---

# 24. CI/CD

Recommended pipelines:

```text
lint
unit-test
integration-test
contract-test
security-scan
build
container-scan
migration-check
deploy-preview
end-to-end-test
deploy-staging
manual-production-approval
deploy-production
post-deploy-smoke-test
```

Use path-based builds so only affected services are rebuilt.

---

# 25. MVP Scope

The MVP should focus on the shortest path from enquiry to enrolment.

## 25.1 MVP modules

- multi-tenant identity and roles;
- institution configuration;
- feature flags;
- programme catalogue;
- CRM;
- lead capture form;
- campaign tracking;
- website AI assistant;
- approved knowledge base;
- email and WhatsApp follow-up;
- application pipeline;
- basic analytics dashboard;
- content generation drafts;
- approval workflow;
- audit logging;
- feedback capture.

## 25.2 Explicitly exclude from MVP

- autonomous advertising spend;
- voice calling agent;
- complex competitor scraping;
- custom ML lead scoring;
- automatic model retraining;
- full alumni platform;
- advanced predictive forecasting;
- dozens of social network integrations;
- custom mobile applications.

---

# 26. Delivery Roadmap

## Phase 0: Foundation

- confirm current AuraEDU architecture;
- define bounded contexts;
- create architecture decision records;
- standardise tenant context;
- standardise auth and permissions;
- establish event contracts;
- create observability baseline.

## Phase 1: Recruitment CRM

- leads;
- pipelines;
- assignments;
- interaction timeline;
- source attribution;
- basic dashboard;
- lead import;
- role permissions.

## Phase 2: Public Conversion Tools

- landing pages;
- programme catalogue;
- enquiry forms;
- website chatbot;
- programme recommendation rules;
- knowledge management.

## Phase 3: Nurturing and Admissions

- email journeys;
- WhatsApp journeys;
- application progress;
- reminders;
- admissions pipeline;
- offer acceptance tracking.

## Phase 4: Content and Campaigns

- campaign builder;
- content generation;
- approval workflow;
- scheduled publishing;
- UTM tracking;
- campaign analytics.

## Phase 5: Intelligence

- executive dashboard;
- funnel diagnostics;
- rules-based lead scoring;
- enrolment forecasts;
- anomaly detection;
- recommendation engine.

## Phase 6: Advanced Automation

- voice agent;
- supervised lead-scoring model;
- reputation intelligence;
- competitor intelligence;
- alumni referral engine;
- controlled autonomous actions.

---

# 27. Initial Product Backlog

## Epic: CRM Foundation

- create lead;
- import leads;
- deduplicate lead;
- assign lead;
- update stage;
- log interaction;
- create follow-up task;
- filter leads;
- search leads;
- export authorised leads;
- view pipeline metrics.

## Epic: AI Admissions Assistant

- upload approved knowledge;
- approve knowledge source;
- chat with assistant;
- cite answer source;
- capture lead from chat;
- escalate conversation;
- collect feedback;
- review unanswered questions.

## Epic: Campaign Management

- create campaign;
- define audience;
- generate tracking link;
- generate draft content;
- request approval;
- approve content;
- schedule campaign;
- record campaign metrics.

## Epic: Admissions Conversion

- start application;
- save progress;
- upload document;
- validate checklist;
- remind applicant;
- submit application;
- update review state;
- issue approved offer;
- record acceptance;
- record enrolment.

---

# 28. Acceptance Criteria Examples

## Website assistant

Given a prospect asks about a programme, when approved institution knowledge contains the answer, then the assistant must answer using only the current tenant's approved content and provide a source reference.

Given the assistant cannot find sufficient approved evidence, when a prospect asks a question, then the assistant must state that it is unsure and offer human escalation.

## Lead creation

Given a prospect submits a form, when the email or phone matches an existing lead under the same tenant, then the system must update or flag the existing record rather than silently create an uncontrolled duplicate.

## Campaign publication

Given a campaign requires approval, when a marketing officer attempts to publish it without approval, then the system must reject the action and create an audit event.

## Tenant isolation

Given a user belongs to Tenant A, when the user attempts to request a Tenant B lead identifier, then the API must return an authorised-safe not-found response and produce a security audit event.

---

# 29. Codex and Agent Execution Instructions

An engineering agent working in this repository must follow these rules.

## 29.1 Before coding

1. Read this specification.
2. Read the root README.
3. Inspect existing architecture and conventions.
4. Read relevant architecture decision records.
5. Identify the bounded context being changed.
6. Identify tenant and permission requirements.
7. Identify event and API contract impact.
8. Write or update tests before declaring completion.

## 29.2 Change constraints

- do not introduce a new framework without justification;
- do not bypass tenant filters;
- do not create cross-service database access;
- do not place business logic in HTTP handlers;
- do not publish events before transaction success;
- use transactional outbox where required;
- do not expose provider-specific details in domain code;
- do not hardcode tenant configuration;
- do not let AI tools execute unrestricted actions;
- do not merge generated code without tests.

## 29.3 Required output for every implementation task

The agent should report:

```text
Summary
Files changed
Architecture decisions
Database migrations
API changes
Event changes
Security impact
Tests added
How to run
Known limitations
```

---

# 30. Suggested First Implementation Slice

Build one complete vertical slice before building the whole platform.

Recommended first slice:

```text
Prospect visits programme page
→ asks website assistant a question
→ submits contact details
→ lead is created in CRM
→ admissions officer sees lead
→ system sends approved welcome email
→ interaction appears in lead timeline
→ dashboard increments lead count
→ prospect feedback is captured
```

Services involved:

- tenant;
- identity;
- CRM;
- knowledge;
- AI orchestrator;
- communication;
- analytics;
- audit.

This slice validates the product's central value while keeping scope controlled.

---

# 31. Definition of Done

A feature is complete only when:

- acceptance criteria pass;
- tenant isolation is tested;
- permissions are tested;
- audit events are emitted;
- API documentation is updated;
- migrations are reversible or safely forward-only;
- logs and metrics exist;
- failure states are handled;
- user interface is accessible;
- AI behaviour is evaluated where relevant;
- deployment instructions are updated;
- no secrets are committed;
- product documentation is updated.

---

# 32. Final Recommendation

Treat AuraEDU Growth as a first-class domain inside AuraEDU.

Use one monorepo now, with clean bounded contexts and independently deployable services. Avoid creating a separate product repository because identity, tenancy, programmes, admissions, notifications, billing, analytics, and AI governance are shared platform capabilities.

The most important architectural principle is not the number of services. It is the quality of the boundaries.

Start with the complete enquiry-to-lead vertical slice, prove that institutions gain measurable admissions value, and then expand into campaigns, applicant nurturing, analytics, and controlled AI automation.
