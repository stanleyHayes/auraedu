# AuraEDU Microservices Multi-Tenant SaaS Platform Specification

**Version:** 2.0  
**Prepared for:** Stanley Asoku Hayford  
**Prepared on:** 2026-07-08  
**Product type:** Multi-tenant, feature-configurable, microservices-based SaaS platform for schools  

---

## 1. Product Direction

AuraEDU must be built as a **multi-tenant SaaS school operating system** using a **microservices architecture**.

UPSHS and Aboom AME Zion C Basic School should not be treated as separate codebases. They should be the first tenants on the same platform.

Each school must have:

- Its own branded portal.
- Its own logo, colours, domain and settings.
- Its own users, students, staff, parents, classes and reports.
- Its own enabled or disabled features.
- Its own academic structure, grading system and payment settings.
- Tenant-isolated data.
- Optional school-specific AI models and insights.

The platform must allow each feature/module to be turned on or off separately for each school.

---

## 2. Core SaaS Principles

1. One platform serves many schools.
2. Schools are tenants.
3. Features are controlled by tenant-level feature flags.
4. Every school has isolated data.
5. No school-specific hardcoding is allowed.
6. New schools must be onboarded through configuration.
7. Microservices own their data and expose APIs/events.
8. AI services must respect tenant boundaries.
9. Billing should be subscription-based.
10. The platform must scale from two schools to hundreds.

---

## 3. Tenant Feature Management

### 3.1 Feature Flag Requirement

Every major feature must be independently enabled or disabled per school.

Examples:

| Feature | UPSHS | Aboom AME Zion C Basic School |
|---|---:|---:|
| Public Website | Enabled | Enabled |
| Admissions | Enabled | Disabled |
| Parent Portal | Enabled | Enabled |
| Student Portal | Enabled | Optional |
| Teacher Portal | Enabled | Enabled |
| Fees | Enabled | Enabled |
| Online Payments | Optional | Disabled |
| CBT / Online Exams | Enabled | Disabled |
| Library | Optional | Disabled |
| Hostel | Disabled | Disabled |
| AI Recommendations | Enabled | Optional |
| Career Guidance | Enabled | Disabled |
| WhatsApp Notifications | Optional | Optional |

### 3.2 Feature Flag Table

```sql
CREATE TABLE tenant_features (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    feature_key VARCHAR(100) NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    plan_required VARCHAR(50),
    config JSONB,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE (tenant_id, feature_key)
);
```

### 3.3 Feature Flag Rules

- Backend APIs must check whether a feature is enabled before allowing access.
- Frontend navigation must hide disabled modules.
- Disabled modules must not appear in dashboards.
- Background jobs must not run disabled modules for a tenant.
- AI models must not use disabled module data.
- Billing must control which features a tenant can enable.
- Platform super admins can override features where necessary.
- Feature flags must support gradual rollout.

### 3.4 Feature Flag Examples

```json
{
  "tenant_id": "upshs",
  "feature_key": "ai_recommendations",
  "is_enabled": true,
  "config": {
    "recommendation_frequency": "weekly",
    "teacher_approval_required": true
  }
}
```

---

## 4. Microservices Architecture

### 4.1 Recommended Services

| Service | Responsibility |
|---|---|
| API Gateway | Single entry point for frontend clients |
| Identity Service | Authentication, sessions, users, roles |
| Tenant Service | Schools, branding, tenant settings, feature flags |
| Student Service | Students, parents, guardians, enrollment records |
| Staff Service | Teachers and non-teaching staff |
| Academic Service | Academic years, terms, classes, subjects, curriculum |
| Attendance Service | Daily and subject attendance |
| Assessment Service | Assignments, tests, exams, scores |
| Report Service | Report cards, transcripts, PDF generation |
| Fees Service | Fees, invoices, balances, receipts |
| Payment Service | Payment gateway integrations and webhooks |
| Notification Service | Email, SMS, WhatsApp, in-app notifications |
| Website Service | Public school websites and content pages |
| File Service | Uploads, documents, images, certificates |
| Analytics Service | Dashboards, KPIs, reporting metrics |
| AI Recommendation Service | Learning recommendations and intervention suggestions |
| AI Prediction Service | Student risk and performance predictions |
| Career Guidance Service | Career path and course recommendations |
| Billing Service | SaaS plans, subscriptions, invoices |
| Audit Service | Audit logs and compliance events |

### 4.2 Communication Pattern

Use both synchronous and asynchronous communication.

Synchronous:
- REST APIs through API Gateway.
- Internal service-to-service calls for direct reads.

Asynchronous:
- Event bus for domain events.
- Background workers for reports, notifications, AI processing and analytics.

Recommended event bus:
- Start simple: NATS or RabbitMQ.
- Later scale: Kafka if event volume becomes very high.

### 4.3 Example Domain Events

```json
{
  "event_type": "assessment.score_recorded",
  "tenant_id": "upshs",
  "student_id": "student-uuid",
  "subject_id": "subject-uuid",
  "score": 72,
  "occurred_at": "2026-07-08T10:00:00Z"
}
```

Important events:
- tenant.created
- tenant.feature_enabled
- tenant.feature_disabled
- student.enrolled
- attendance.marked
- assessment.score_recorded
- report.published
- invoice.created
- payment.received
- ai.recommendation_generated
- user.role_changed

---

## 5. Tenant Isolation in Microservices

### 5.1 Tenant Context

Every request and event must include tenant context.

Required tenant context fields:
- tenant_id
- tenant_code
- request_id
- actor_user_id
- actor_role
- feature_flags_snapshot where needed

### 5.2 Database Isolation

Recommended initial approach:
- Database per service.
- Shared schema per service.
- Every tenant-owned table includes `tenant_id`.

Example:
```text
identity_db
tenant_db
student_db
academic_db
attendance_db
assessment_db
fees_db
notification_db
ai_db
billing_db
```

Enterprise future option:
- Dedicated database per large school or school group.

### 5.3 Tenant Isolation Rules

- All queries must be scoped by tenant_id.
- Services must reject requests without tenant context.
- Events without tenant_id must be rejected.
- Cache keys must include tenant_id.
- File paths must include tenant code.
- Audit logs must include tenant_id.
- AI training and inference must respect tenant boundaries.

---

## 6. Service Ownership and Data Boundaries

Each microservice owns its own database. Other services must not directly access another service's database.

Correct:
```text
Assessment Service -> publishes assessment.score_recorded event
Analytics Service -> consumes event and updates analytics projections
AI Service -> consumes event and updates feature store
```

Wrong:
```text
AI Service -> directly queries Assessment Service database
```

---

## 7. API Gateway

The API Gateway handles:

- Routing to services.
- Authentication verification.
- Tenant resolution.
- Rate limiting.
- Request logging.
- Request ID generation.
- Basic request validation.
- CORS.
- API versioning.

API path examples:
```text
/api/v1/tenants
/api/v1/students
/api/v1/attendance
/api/v1/assessments
/api/v1/reports
/api/v1/features
/api/v1/ai/recommendations
```

---

## 8. Identity, RBAC and Permissions

### 8.1 Roles

| Role | Scope |
|---|---|
| Platform Super Admin | All tenants |
| School Admin | Single tenant |
| Principal/Headteacher | Single tenant |
| Academic Head | Single tenant |
| Accountant | Single tenant |
| Teacher | Assigned classes/subjects |
| Parent | Own children only |
| Student | Own records only |
| Support Agent | Limited platform support |

### 8.2 Permission Format

Use permission keys:

```text
students.read
students.create
students.update
attendance.mark
assessments.record_scores
reports.publish
fees.manage
features.manage
ai.view_recommendations
```

### 8.3 Authorization Rule

Access is granted only when:

```text
authenticated == true
tenant_id matches request tenant
role has required permission
feature is enabled for tenant
resource belongs to tenant
```

---

## 9. Feature Catalogue

Feature keys should be stable and controlled.

```text
public_website
admissions
student_management
staff_management
parent_portal
student_portal
teacher_portal
attendance
assignments
assessments
cbt_exams
report_cards
fees
online_payments
timetable
library
hostel
transport
announcements
email_notifications
sms_notifications
whatsapp_notifications
analytics
ai_recommendations
ai_predictions
career_guidance
billing
custom_domain
```

---

## 10. Frontend Behaviour

The frontend must be tenant-aware and feature-aware.

Rules:
- Load tenant configuration at startup.
- Apply tenant theme dynamically.
- Show only enabled modules.
- Hide disabled menu items.
- Prevent direct route access to disabled modules.
- Display upgrade messages for paid but disabled features where appropriate.
- Use shared UI components.
- Do not hardcode school names or colours.

Example:
```tsx
if (!features.ai_recommendations) {
  return <FeatureUnavailable feature="AI Recommendations" />;
}
```

---

## 11. Recommended Monorepo Structure

```text
auraedu/
  apps/
    web/
    api-gateway/
    identity-service/
    tenant-service/
    student-service/
    staff-service/
    academic-service/
    attendance-service/
    assessment-service/
    report-service/
    fees-service/
    payment-service/
    notification-service/
    website-service/
    analytics-service/
    billing-service/
    ai-recommendation-service/
    ai-prediction-service/
    career-guidance-service/
  packages/
    ui/
    shared-types/
    config/
    logger/
    eslint-config/
  contracts/
    openapi/
    events/
  infrastructure/
    docker/
    kubernetes/
    terraform/
  docs/
    architecture/
    api/
    onboarding/
    ai/
    runbooks/
  scripts/
  .github/
    workflows/
  docker-compose.yml
  README.md
```

---

## 12. Service Internal Structure

Each Go service should follow Hexagonal Architecture.

```text
student-service/
  cmd/
    server/
      main.go
    worker/
      main.go
  internal/
    domain/
      student.go
      guardian.go
      errors.go
    application/
      create_student.go
      update_student.go
      list_students.go
    ports/
      repository.go
      event_publisher.go
    adapters/
      postgres/
      http/
      events/
    platform/
      config/
      logger/
      auth/
      tenancy/
  migrations/
  tests/
  Dockerfile
```

Rules:
- Domain layer contains business rules.
- Application layer contains use cases.
- Ports define interfaces.
- Adapters implement interfaces.
- HTTP handlers must not contain business logic.
- Tenant enforcement must exist in application and repository layers.

---

## 13. CI/CD Requirements

Each service must have an independent build and test pipeline.

Pull request pipeline:
- Detect changed services.
- Run formatting.
- Run linting.
- Run type checks.
- Run unit tests.
- Run integration tests.
- Run contract tests.
- Build Docker images.
- Run security scan.
- Validate migrations.

Main branch pipeline:
- Build changed services.
- Push images to registry.
- Deploy to staging.
- Run smoke tests.
- Run tenant isolation tests.
- Manual approval for production.
- Deploy to production.
- Run post-deployment checks.

---

## 14. Testing Strategy

### 14.1 Required Tests

| Test Type | Purpose |
|---|---|
| Unit tests | Business logic |
| Integration tests | Database and adapters |
| Contract tests | Service API compatibility |
| E2E tests | Full user flows |
| Tenant isolation tests | Prevent cross-school data access |
| Feature flag tests | Confirm disabled features are blocked |
| Security tests | Auth, RBAC, injection, file upload |
| Performance tests | Load and scaling |
| AI evaluation tests | Model quality and safety |

### 14.2 Critical Feature Flag Tests

- Disabled module does not appear in frontend.
- Disabled module route cannot be accessed directly.
- Disabled module API returns forbidden or feature-disabled error.
- Disabled module background jobs do not run.
- Disabled module events are ignored where appropriate.
- School A can enable a feature while School B has it disabled.

---

## 15. AI Architecture

AI should be implemented as separate Python microservices.

AI services:
- AI Recommendation Service
- AI Prediction Service
- Career Guidance Service

AI data flow:
1. Assessment Service publishes scores.
2. Attendance Service publishes attendance.
3. Analytics Service builds tenant-level learning metrics.
4. AI services consume approved tenant data.
5. AI services generate recommendations.
6. Teacher reviews AI recommendations.
7. Parent and student see approved guidance.

AI rules:
- AI must be explainable.
- Teachers can override recommendations.
- Recommendations must show confidence levels.
- No AI output should be treated as final academic truth.
- Models may be trained per tenant.
- Cross-school training must use anonymized data only.
- AI modules must obey feature flags.

---

## 16. Subscription and Billing

Billing Service controls plans and feature access.

Example plan mapping:

| Plan | Features |
|---|---|
| Starter | Website, students, parents, attendance, reports |
| Growth | Starter + fees, assessments, messaging |
| Professional | Growth + CBT, analytics, custom domain |
| AI Plus | Professional + AI recommendations, predictions, career guidance |
| Enterprise | AI Plus + custom integrations and SLA |

If a school downgrades:
- Data should not be deleted immediately.
- Disabled features become read-only or hidden based on policy.
- Admin receives warning before access changes.
- Billing history remains available.

---

## 17. Observability

Every service must emit:
- Structured logs.
- Metrics.
- Traces.
- Health checks.
- Readiness checks.
- Tenant-aware audit logs.

Recommended tools:
- OpenTelemetry
- Prometheus
- Grafana
- Loki
- Sentry or similar error tracking

Important metrics:
- API latency per service.
- Error rate per service.
- Failed login attempts.
- Tenant usage.
- Notification delivery rate.
- Payment webhook failures.
- AI service latency.
- Background job failures.

---

## 18. Deployment Architecture

Recommended early MVP:
- Docker Compose for local development.
- Managed PostgreSQL.
- Managed Redis.
- Containerized services.
- GitHub Actions CI/CD.

Recommended production:
- Kubernetes.
- Ingress controller.
- API Gateway.
- Separate namespaces for staging and production.
- Horizontal autoscaling.
- Secret manager.
- Managed database backups.
- Object storage.
- Monitoring stack.

---

## 19. School Onboarding Flow

1. Create tenant.
2. Choose subscription plan.
3. Enable selected features.
4. Upload logo and colours.
5. Configure domain or subdomain.
6. Configure academic year and terms.
7. Configure classes and subjects.
8. Configure grading system.
9. Import students, parents and staff.
10. Assign teachers to classes and subjects.
11. Configure fees if enabled.
12. Configure website pages.
13. Train staff.
14. Go live.

---

## 20. Initial Tenants

### 20.1 UPSHS - University Practice Senior High School

Tenant code:
```text
upshs
```

Suggested enabled features:
- Public Website
- Admissions
- Student Management
- Staff Management
- Parent Portal
- Student Portal
- Teacher Portal
- Attendance
- Assessments
- Report Cards
- Fees
- Analytics
- AI Recommendations
- AI Predictions
- Career Guidance

### 20.2 Aboom AME Zion C Basic School

Tenant code:
```text
aboom-ame-zion-c
```

Suggested enabled features:
- Public Website
- Student Management
- Staff Management
- Parent Portal
- Teacher Portal
- Attendance
- Assessments
- Report Cards
- Fees
- Announcements

Optional later features:
- Student Portal
- Online Payments
- CBT Exams
- Analytics
- AI Recommendations

---

## 21. Definition of Done

A feature is complete only when:
- It is tenant-aware.
- It is controlled by feature flags.
- It has RBAC permissions.
- APIs are documented in OpenAPI.
- Events are documented where applicable.
- Unit tests pass.
- Integration tests pass.
- Contract tests pass where applicable.
- Tenant isolation tests pass.
- Feature flag tests pass.
- Frontend handles enabled and disabled states.
- Logs and audit events are implemented.
- CI/CD pipeline passes.
- Staging smoke tests pass.

---

## 22. AI Coding Agent Instructions

Any AI agent or engineer must follow these rules:

1. Do not create separate codebases for different schools.
2. Do not hardcode school names.
3. Every school-specific behaviour must be tenant configuration or feature flags.
4. Each microservice owns its database.
5. Services must communicate through APIs or events, not direct database access.
6. Every tenant-owned table must include tenant_id.
7. Every tenant-owned event must include tenant_id.
8. Every protected action must check authentication, tenant scope, RBAC and feature flag.
9. Disabled features must be blocked in frontend, backend and workers.
10. Add tests for tenant isolation and feature flag behaviour.
11. Use Hexagonal Architecture inside each service.
12. Keep business logic out of HTTP handlers.
13. Update OpenAPI and event contracts when interfaces change.
14. Never log sensitive student data.
15. Use secure defaults.

---

## 23. Conclusion

AuraEDU should be built as a microservices-based, multi-tenant SaaS platform with per-school feature configuration. UPSHS and Aboom AME Zion C Basic School become the first tenants, not separate applications.

This architecture allows every school to have its own branded experience while running on one scalable platform. It also allows features such as AI recommendations, CBT, payments, library, hostel and career guidance to be enabled or disabled independently for each school.
