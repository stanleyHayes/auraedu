#!/usr/bin/env python3
"""Seed the remaining OpenAPI + CloudEvents contract skeletons for AuraEDU Sprint 0.

Run from the repo root:
    python3 tools/codegen/scripts/seed_contracts.py

Existing files are never overwritten.
"""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
OPENAPI_DIR = ROOT / "contracts" / "openapi"
EVENTS_DIR = ROOT / "contracts" / "events"

STANDARD_ERRORS = """components:
  parameters:
    TenantId:
      name: tenant_id
      in: path
      required: true
      schema: { type: string, format: uuid }
    TenantHeader:
      name: X-Tenant-Code
      in: header
      required: false
      description: Optional tenant code for resolution when the gateway cannot derive it from the host.
      schema: { type: string, example: upshs }
    Limit:
      name: limit
      in: query
      schema: { type: integer, minimum: 1, maximum: 100, default: 25 }
    Cursor:
      name: cursor
      in: query
      schema: { type: string }

  schemas:
    Error:
      type: object
      required: [code, message]
      properties:
        code: { type: string, enum: [forbidden, feature_disabled, tenant_mismatch, validation_error, not_found, unauthorized] }
        message: { type: string }
        request_id: { type: string }

  responses:
    Unauthorized:
      description: Missing or invalid bearer token
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
          example: { code: unauthorized, message: "Authentication required" }
    Forbidden:
      description: Not permitted (auth, tenant scope, RBAC, or feature disabled)
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
          examples:
            forbidden: { value: { code: forbidden, message: "Missing required permission" } }
            feature_disabled: { value: { code: feature_disabled, message: "Feature not enabled for tenant" } }
            tenant_mismatch: { value: { code: tenant_mismatch, message: "Resource belongs to another tenant" } }
    NotFound:
      description: Resource not found
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
          example: { code: not_found, message: "Resource not found" }
    ValidationError:
      description: Request failed validation
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
          example: { code: validation_error, message: "request body is invalid" }

  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

security:
  - bearerAuth: []
"""


def indent(text: str, width: int = 4) -> str:
    return "\n".join(" " * width + line if line else "" for line in text.splitlines())


def make_openapi(
    key: str,
    title: str,
    description: str,
    tags: list[str],
    paths: str,
    schemas: str,
) -> str:
    tag_lines = "\n".join(f"  - name: {t}" for t in tags)
    extra_schemas = indent(schemas, width=4).rstrip() if schemas.strip() else ""
    return f"""openapi: 3.1.0
info:
  title: {title}
  version: 1.0.0
  description: >
    {description}
servers:
  - url: /api/v1
tags:
{tag_lines}

paths:
{indent(paths, width=2)}

{STANDARD_ERRORS}
{extra_schemas}
"""


def op(
    method: str,
    operation_id: str,
    summary: str,
    tags: list[str],
    params: list[str] | None = None,
    request: str | None = None,
    response: str | None = None,
    response_code: str = "200",
    extra_responses: list[str] | None = None,
    no_auth: bool = False,
) -> str:
    """Render a single operation."""
    tag_list = ", ".join(f"{t!r}" for t in tags)
    lines = [
        f"{method}:",
        f"      operationId: {operation_id}",
        f"      tags: [{tag_list}]",
        f"      summary: {summary}",
    ]
    all_params = ["$ref: '#/components/parameters/TenantHeader'"]
    if params:
        all_params.extend(params)
    lines.append("      parameters:")
    for p in all_params:
        lines.append(f"        - {p}")
    if request:
        lines.extend([
            "      requestBody:",
            "        required: true",
            "        content:",
            "          application/json:",
            f"            schema: {{ $ref: '#/components/schemas/{request}' }}",
        ])
    lines.append("      responses:")
    if response:
        lines.append(f"        '{response_code}':")
        lines.append("          description: OK" if response_code.startswith("2") else "          description: Accepted")
        lines.append("          content:")
        lines.append("            application/json:")
        lines.append(f"              schema: {{ $ref: '#/components/schemas/{response}' }}")
    else:
        lines.append(f"        '{response_code}':")
        lines.append("          description: No content")
    for r in (["'403': { $ref: '#/components/responses/Forbidden' }", "'404': { $ref: '#/components/responses/NotFound' }", "'422': { $ref: '#/components/responses/ValidationError' }"] + (extra_responses or [])):
        lines.append(f"        {r}")
    lines.append("        '401': { $ref: '#/components/responses/Unauthorized' }")
    if no_auth:
        lines.append("      security: []")
    return "\n".join(f"    {line}" for line in lines)


def res(schema: str, is_array: bool = False) -> str:
    if is_array:
        return f"""type: object
properties:
  data: {{ type: array, items: {{ $ref: '#/components/schemas/{schema}' }} }}
  next_cursor: {{ type: [string, 'null'] }}"""
    return f"allOf:\n  - {{ $ref: '#/components/schemas/{schema}' }}"


def schema(name: str, body: str) -> str:
    return f"{name}:\n{indent(body, width=4)}"


OPENAPI_SPECS: list[dict] = [
    {
        "key": "identity",
        "title": "AuraEDU Identity Service API",
        "description": "Authentication, users, roles, and sessions (spec \\u00a77). Owned by lane L1 (EP-04).",
        "tags": ["auth", "users", "roles", "sessions"],
        "paths": [
            ("/auth/login", op("post", "login", "Exchange credentials for JWT tokens", ["auth"], request="LoginRequest", response="TokenPair", no_auth=True)),
            ("/auth/refresh", op("post", "refreshToken", "Rotate access token using refresh token", ["auth"], request="RefreshRequest", response="TokenPair", no_auth=True)),
            ("/auth/sessions/{session_id}", op("delete", "revokeSession", "Revoke a session", ["sessions"], params=["$ref: '#/components/parameters/TenantId'"], response_code="204")),
            ("/users", op("get", "listUsers", "List users for the tenant", ["users"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="UserList")),
            ("/users", op("post", "createUser", "Create a user in the tenant", ["users"], request="CreateUser", response="User", response_code="201")),
            ("/users/{user_id}", op("get", "getUser", "Get a user", ["users"], params=["$ref: '#/components/parameters/TenantId'"], response="User")),
            ("/users/{user_id}", op("patch", "updateUser", "Update a user", ["users"], params=["$ref: '#/components/parameters/TenantId'"], request="UpdateUser", response="User")),
            ("/users/{user_id}/roles", op("post", "assignRole", "Assign a role to a user", ["users", "roles"], params=["$ref: '#/components/parameters/TenantId'"], request="RoleAssignment", response="User")),
            ("/roles", op("get", "listRoles", "List available roles", ["roles"], response="RoleList")),
        ],
        "schemas": [
            schema("LoginRequest", "type: object\nrequired: [identifier, password]\nproperties:\n  identifier: { type: string }\n  password: { type: string, format: password }"),
            schema("RefreshRequest", "type: object\nrequired: [refresh_token]\nproperties:\n  refresh_token: { type: string }"),
            schema("TokenPair", "type: object\nrequired: [access_token, refresh_token, expires_in]\nproperties:\n  access_token: { type: string }\n  refresh_token: { type: string }\n  expires_in: { type: integer }\n  token_type: { type: string, default: Bearer }"),
            schema("User", "type: object\nrequired: [id, tenant_id, email, role, status]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  email: { type: string, format: email }\n  username: { type: [string, 'null'] }\n  role: { type: string }\n  status: { type: string, enum: [active, suspended, pending] }\n  created_at: { type: string, format: date-time }\n  updated_at: { type: string, format: date-time }"),
            schema("CreateUser", "type: object\nrequired: [email, role]\nproperties:\n  email: { type: string, format: email }\n  username: { type: [string, 'null'] }\n  role: { type: string }\n  password: { type: string, format: password }"),
            schema("UpdateUser", "type: object\nproperties:\n  email: { type: string, format: email }\n  username: { type: [string, 'null'] }\n  role: { type: string }\n  status: { type: string, enum: [active, suspended, pending] }"),
            schema("RoleAssignment", "type: object\nrequired: [role]\nproperties:\n  role: { type: string }\n  permissions: { type: array, items: { type: string } }"),
            schema("Role", "type: object\nrequired: [key, name]\nproperties:\n  key: { type: string }\n  name: { type: string }\n  permissions: { type: array, items: { type: string } }"),
            schema("UserList", res("User", is_array=True)),
            schema("RoleList", res("Role", is_array=True)),
        ],
    },
    {
        "key": "gateway",
        "title": "AuraEDU API Gateway API",
        "description": "Routing, health, and service registry surface exposed by the gateway itself (spec \\u00a77). Owned by lane L1 (EP-03).",
        "tags": ["health", "routes"],
        "paths": [
            ("/health", op("get", "healthCheck", "Gateway health", ["health"], response="Health")),
            ("/ready", op("get", "readyCheck", "Gateway readiness", ["health"], response="Health")),
            ("/routes", op("get", "listRoutes", "List configured upstream routes", ["routes"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="RouteList")),
            ("/routes", op("post", "createRoute", "Register an upstream route", ["routes"], request="CreateRoute", response="Route", response_code="201")),
        ],
        "schemas": [
            schema("Health", "type: object\nrequired: [status]\nproperties:\n  status: { type: string, enum: [ok, degraded, down] }\n  checks: { type: object, additionalProperties: true }"),
            schema("Route", "type: object\nrequired: [path_prefix, upstream]\nproperties:\n  path_prefix: { type: string }\n  upstream: { type: string }\n  strip_prefix: { type: boolean, default: true }\n  require_auth: { type: boolean, default: true }"),
            schema("CreateRoute", "type: object\nrequired: [path_prefix, upstream]\nproperties:\n  path_prefix: { type: string }\n  upstream: { type: string }\n  strip_prefix: { type: boolean, default: true }\n  require_auth: { type: boolean, default: true }"),
            schema("RouteList", res("Route", is_array=True)),
        ],
    },
    {
        "key": "student",
        "title": "AuraEDU Student Service API",
        "description": "Students, guardians, and enrollment (spec \\u00a77). Owned by lane L2 (EP-10). Feature flag: student_management.",
        "tags": ["students", "guardians", "enrollments"],
        "paths": [
            ("/students", op("get", "listStudents", "List students", ["students"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="StudentList")),
            ("/students", op("post", "createStudent", "Create a student", ["students"], request="CreateStudent", response="Student", response_code="201")),
            ("/students/{student_id}", op("get", "getStudent", "Get a student", ["students"], params=["$ref: '#/components/parameters/TenantId'"], response="Student")),
            ("/students/{student_id}", op("patch", "updateStudent", "Update a student", ["students"], params=["$ref: '#/components/parameters/TenantId'"], request="UpdateStudent", response="Student")),
            ("/students/{student_id}", op("delete", "deleteStudent", "Delete a student", ["students"], params=["$ref: '#/components/parameters/TenantId'"], response_code="204")),
            ("/students/{student_id}/enrollments", op("get", "listEnrollments", "List student enrollments", ["enrollments"], params=["$ref: '#/components/parameters/TenantId'", "$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="EnrollmentList")),
            ("/students/{student_id}/enrollments", op("post", "createEnrollment", "Enroll a student", ["enrollments"], params=["$ref: '#/components/parameters/TenantId'"], request="CreateEnrollment", response="Enrollment", response_code="201")),
            ("/students/{student_id}/guardians", op("get", "listStudentGuardians", "List a student's guardians", ["guardians"], params=["$ref: '#/components/parameters/TenantId'"], response="GuardianList")),
            ("/guardians/{guardian_id}", op("get", "getGuardian", "Get a guardian", ["guardians"], params=["$ref: '#/components/parameters/TenantId'"], response="Guardian")),
        ],
        "schemas": [
            schema("Student", "type: object\nrequired: [id, tenant_id, first_name, last_name, student_code]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  first_name: { type: string }\n  last_name: { type: string }\n  student_code: { type: string }\n  date_of_birth: { type: [string, 'null'], format: date }\n  gender: { type: [string, 'null'], enum: [male, female, other] }\n  status: { type: string, enum: [active, withdrawn, graduated] }\n  created_at: { type: string, format: date-time }\n  updated_at: { type: string, format: date-time }"),
            schema("CreateStudent", "type: object\nrequired: [first_name, last_name]\nproperties:\n  first_name: { type: string }\n  last_name: { type: string }\n  date_of_birth: { type: [string, 'null'], format: date }\n  gender: { type: [string, 'null'], enum: [male, female, other] }\n  class_id: { type: [string, 'null'], format: uuid }"),
            schema("UpdateStudent", "type: object\nproperties:\n  first_name: { type: string }\n  last_name: { type: string }\n  status: { type: string, enum: [active, withdrawn, graduated] }\n  class_id: { type: [string, 'null'], format: uuid }"),
            schema("Guardian", "type: object\nrequired: [id, tenant_id, first_name, last_name, relationship]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  first_name: { type: string }\n  last_name: { type: string }\n  relationship: { type: string }\n  phone: { type: [string, 'null'] }\n  email: { type: [string, 'null'], format: email }"),
            schema("Enrollment", "type: object\nrequired: [id, student_id, class_id, academic_year_id]\nproperties:\n  id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  class_id: { type: string, format: uuid }\n  academic_year_id: { type: string, format: uuid }\n  enrolled_at: { type: string, format: date-time }"),
            schema("CreateEnrollment", "type: object\nrequired: [class_id, academic_year_id]\nproperties:\n  class_id: { type: string, format: uuid }\n  academic_year_id: { type: string, format: uuid }"),
            schema("StudentList", res("Student", is_array=True)),
            schema("GuardianList", res("Guardian", is_array=True)),
            schema("EnrollmentList", res("Enrollment", is_array=True)),
        ],
    },
    {
        "key": "staff",
        "title": "AuraEDU Staff Service API",
        "description": "Teachers and non-teaching staff (spec \\u00a77). Owned by lane L2 (EP-11). Feature flag: staff_management.",
        "tags": ["staff", "assignments"],
        "paths": [
            ("/staff", op("get", "listStaff", "List staff", ["staff"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="StaffList")),
            ("/staff", op("post", "createStaff", "Create a staff record", ["staff"], request="CreateStaff", response="Staff", response_code="201")),
            ("/staff/{staff_id}", op("get", "getStaff", "Get a staff record", ["staff"], params=["$ref: '#/components/parameters/TenantId'"], response="Staff")),
            ("/staff/{staff_id}", op("patch", "updateStaff", "Update a staff record", ["staff"], params=["$ref: '#/components/parameters/TenantId'"], request="UpdateStaff", response="Staff")),
            ("/staff/{staff_id}/assignments", op("get", "listStaffAssignments", "List staff assignments", ["assignments"], params=["$ref: '#/components/parameters/TenantId'"], response="StaffAssignmentList")),
            ("/staff/{staff_id}/assignments", op("post", "createStaffAssignment", "Assign staff to class/subject", ["assignments"], params=["$ref: '#/components/parameters/TenantId'"], request="CreateStaffAssignment", response="StaffAssignment", response_code="201")),
        ],
        "schemas": [
            schema("Staff", "type: object\nrequired: [id, tenant_id, first_name, last_name, staff_type]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  first_name: { type: string }\n  last_name: { type: string }\n  staff_type: { type: string, enum: [teacher, non_teaching] }\n  email: { type: [string, 'null'], format: email }\n  staff_code: { type: string }\n  created_at: { type: string, format: date-time }"),
            schema("CreateStaff", "type: object\nrequired: [first_name, last_name, staff_type]\nproperties:\n  first_name: { type: string }\n  last_name: { type: string }\n  staff_type: { type: string, enum: [teacher, non_teaching] }\n  email: { type: [string, 'null'], format: email }"),
            schema("UpdateStaff", "type: object\nproperties:\n  first_name: { type: string }\n  last_name: { type: string }\n  staff_type: { type: string, enum: [teacher, non_teaching] }\n  email: { type: [string, 'null'], format: email }"),
            schema("StaffAssignment", "type: object\nrequired: [id, staff_id]\nproperties:\n  id: { type: string, format: uuid }\n  staff_id: { type: string, format: uuid }\n  class_id: { type: [string, 'null'], format: uuid }\n  subject_id: { type: [string, 'null'], format: uuid }\n  role: { type: [string, 'null'] }\n  assigned_at: { type: string, format: date-time }"),
            schema("CreateStaffAssignment", "type: object\nrequired: [class_id]\nproperties:\n  class_id: { type: string, format: uuid }\n  subject_id: { type: [string, 'null'], format: uuid }\n  role: { type: [string, 'null'] }"),
            schema("StaffList", res("Staff", is_array=True)),
            schema("StaffAssignmentList", res("StaffAssignment", is_array=True)),
        ],
    },
    {
        "key": "academic",
        "title": "AuraEDU Academic Service API",
        "description": "Academic years, terms, classes, subjects, curriculum, and grading scales (spec \\u00a77). Owned by lane L2 (EP-12).",
        "tags": ["academic-years", "terms", "classes", "subjects", "grading"],
        "paths": [
            ("/academic-years", op("get", "listAcademicYears", "List academic years", ["academic-years"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="AcademicYearList")),
            ("/academic-years", op("post", "createAcademicYear", "Create an academic year", ["academic-years"], request="CreateAcademicYear", response="AcademicYear", response_code="201")),
            ("/terms", op("get", "listTerms", "List terms", ["terms"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="TermList")),
            ("/terms", op("post", "createTerm", "Create a term", ["terms"], request="CreateTerm", response="Term", response_code="201")),
            ("/classes", op("get", "listClasses", "List classes", ["classes"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="ClassList")),
            ("/classes", op("post", "createClass", "Create a class", ["classes"], request="CreateClass", response="Class", response_code="201")),
            ("/subjects", op("get", "listSubjects", "List subjects", ["subjects"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="SubjectList")),
            ("/subjects", op("post", "createSubject", "Create a subject", ["subjects"], request="CreateSubject", response="Subject", response_code="201")),
            ("/grading-scales", op("get", "listGradingScales", "List grading scales", ["grading"], response="GradingScaleList")),
            ("/grading-scales", op("post", "createGradingScale", "Create a grading scale", ["grading"], request="CreateGradingScale", response="GradingScale", response_code="201")),
        ],
        "schemas": [
            schema("AcademicYear", "type: object\nrequired: [id, tenant_id, name, start_date, end_date]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  start_date: { type: string, format: date }\n  end_date: { type: string, format: date }\n  is_current: { type: boolean }"),
            schema("CreateAcademicYear", "type: object\nrequired: [name, start_date, end_date]\nproperties:\n  name: { type: string }\n  start_date: { type: string, format: date }\n  end_date: { type: string, format: date }\n  is_current: { type: boolean, default: false }"),
            schema("Term", "type: object\nrequired: [id, tenant_id, academic_year_id, name, start_date, end_date]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  academic_year_id: { type: string, format: uuid }\n  name: { type: string }\n  start_date: { type: string, format: date }\n  end_date: { type: string, format: date }"),
            schema("CreateTerm", "type: object\nrequired: [academic_year_id, name, start_date, end_date]\nproperties:\n  academic_year_id: { type: string, format: uuid }\n  name: { type: string }\n  start_date: { type: string, format: date }\n  end_date: { type: string, format: date }"),
            schema("Class", "type: object\nrequired: [id, tenant_id, name, academic_year_id]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  academic_year_id: { type: string, format: uuid }\n  class_teacher_id: { type: [string, 'null'], format: uuid }\n  capacity: { type: [integer, 'null'] }"),
            schema("CreateClass", "type: object\nrequired: [name, academic_year_id]\nproperties:\n  name: { type: string }\n  academic_year_id: { type: string, format: uuid }\n  class_teacher_id: { type: [string, 'null'], format: uuid }\n  capacity: { type: [integer, 'null'] }"),
            schema("Subject", "type: object\nrequired: [id, tenant_id, name]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  code: { type: [string, 'null'] }\n  description: { type: [string, 'null'] }"),
            schema("CreateSubject", "type: object\nrequired: [name]\nproperties:\n  name: { type: string }\n  code: { type: [string, 'null'] }\n  description: { type: [string, 'null'] }"),
            schema("GradingScale", "type: object\nrequired: [id, tenant_id, name]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  ranges: { type: array, items: { type: object, properties: { min: { type: number }, max: { type: number }, grade: { type: string }, remark: { type: string } } } }"),
            schema("CreateGradingScale", "type: object\nrequired: [name]\nproperties:\n  name: { type: string }\n  ranges: { type: array, items: { type: object, properties: { min: { type: number }, max: { type: number }, grade: { type: string }, remark: { type: string } } } }"),
            schema("AcademicYearList", res("AcademicYear", is_array=True)),
            schema("TermList", res("Term", is_array=True)),
            schema("ClassList", res("Class", is_array=True)),
            schema("SubjectList", res("Subject", is_array=True)),
            schema("GradingScaleList", res("GradingScale", is_array=True)),
        ],
    },
    {
        "key": "attendance",
        "title": "AuraEDU Attendance Service API",
        "description": "Daily and subject attendance (spec \\u00a77). Owned by lane L2 (EP-13). Feature flag: attendance.",
        "tags": ["attendance"],
        "paths": [
            ("/attendance", op("get", "listAttendance", "List attendance records", ["attendance"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="AttendanceRecordList")),
            ("/attendance/bulk", op("post", "markAttendanceBulk", "Mark attendance in bulk", ["attendance"], request="BulkAttendanceRequest", response="AttendanceRecordList", response_code="201")),
            ("/students/{student_id}/attendance", op("get", "getStudentAttendance", "Get attendance for a student", ["attendance"], params=["$ref: '#/components/parameters/TenantId'", "$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="AttendanceRecordList")),
        ],
        "schemas": [
            schema("AttendanceRecord", "type: object\nrequired: [id, tenant_id, student_id, date, status]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  class_id: { type: [string, 'null'], format: uuid }\n  subject_id: { type: [string, 'null'], format: uuid }\n  date: { type: string, format: date }\n  status: { type: string, enum: [present, absent, late, excused] }\n  recorded_by: { type: string, format: uuid }\n  recorded_at: { type: string, format: date-time }"),
            schema("BulkAttendanceRequest", "type: object\nrequired: [date, records]\nproperties:\n  date: { type: string, format: date }\n  class_id: { type: [string, 'null'], format: uuid }\n  subject_id: { type: [string, 'null'], format: uuid }\n  records: { type: array, items: { type: object, required: [student_id, status], properties: { student_id: { type: string, format: uuid }, status: { type: string, enum: [present, absent, late, excused] }, remark: { type: [string, 'null'] } } } }"),
            schema("AttendanceRecordList", res("AttendanceRecord", is_array=True)),
        ],
    },
    {
        "key": "assessment",
        "title": "AuraEDU Assessment Service API",
        "description": "Assignments, tests, exams, and scores (spec \\u00a77). Owned by lane L2 (EP-14). Feature flags: assessments, assignments.",
        "tags": ["assessments", "scores", "assignments"],
        "paths": [
            ("/assessments", op("get", "listAssessments", "List assessments", ["assessments"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="AssessmentList")),
            ("/assessments", op("post", "createAssessment", "Create an assessment", ["assessments"], request="CreateAssessment", response="Assessment", response_code="201")),
            ("/assessments/{assessment_id}", op("get", "getAssessment", "Get an assessment", ["assessments"], params=["$ref: '#/components/parameters/TenantId'"], response="Assessment")),
            ("/assessments/{assessment_id}/scores", op("post", "recordScore", "Record a score", ["scores"], params=["$ref: '#/components/parameters/TenantId'"], request="CreateScore", response="Score", response_code="201")),
            ("/assignments", op("get", "listAssignments", "List assignments", ["assignments"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="AssignmentList")),
            ("/assignments", op("post", "createAssignment", "Create an assignment", ["assignments"], request="CreateAssignment", response="Assignment", response_code="201")),
        ],
        "schemas": [
            schema("Assessment", "type: object\nrequired: [id, tenant_id, name, type, subject_id]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  type: { type: string, enum: [test, exam, quiz] }\n  subject_id: { type: string, format: uuid }\n  class_id: { type: [string, 'null'], format: uuid }\n  term_id: { type: [string, 'null'], format: uuid }\n  max_score: { type: number, minimum: 0 }\n  scheduled_at: { type: [string, 'null'], format: date-time }"),
            schema("CreateAssessment", "type: object\nrequired: [name, type, subject_id]\nproperties:\n  name: { type: string }\n  type: { type: string, enum: [test, exam, quiz] }\n  subject_id: { type: string, format: uuid }\n  class_id: { type: [string, 'null'], format: uuid }\n  term_id: { type: [string, 'null'], format: uuid }\n  max_score: { type: number, minimum: 0 }\n  scheduled_at: { type: [string, 'null'], format: date-time }"),
            schema("Score", "type: object\nrequired: [id, assessment_id, student_id, score]\nproperties:\n  id: { type: string, format: uuid }\n  assessment_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  score: { type: number, minimum: 0 }\n  max_score: { type: [number, 'null'], minimum: 0 }\n  recorded_by: { type: [string, 'null'], format: uuid }"),
            schema("CreateScore", "type: object\nrequired: [student_id, score]\nproperties:\n  student_id: { type: string, format: uuid }\n  score: { type: number, minimum: 0 }\n  max_score: { type: [number, 'null'], minimum: 0 }"),
            schema("Assignment", "type: object\nrequired: [id, tenant_id, title, subject_id]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  title: { type: string }\n  subject_id: { type: string, format: uuid }\n  class_ids: { type: array, items: { type: string, format: uuid } }\n  due_date: { type: [string, 'null'], format: date-time }\n  published_at: { type: [string, 'null'], format: date-time }"),
            schema("CreateAssignment", "type: object\nrequired: [title, subject_id]\nproperties:\n  title: { type: string }\n  subject_id: { type: string, format: uuid }\n  class_ids: { type: array, items: { type: string, format: uuid } }\n  due_date: { type: [string, 'null'], format: date-time }"),
            schema("AssessmentList", res("Assessment", is_array=True)),
            schema("AssignmentList", res("Assignment", is_array=True)),
        ],
    },
    {
        "key": "report",
        "title": "AuraEDU Report Service API",
        "description": "Report cards, transcripts, and templates (spec \\u00a77). Owned by lane L2 (EP-15). Feature flag: report_cards.",
        "tags": ["report-cards", "transcripts", "templates"],
        "paths": [
            ("/report-cards", op("get", "listReportCards", "List report cards", ["report-cards"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="ReportCardList")),
            ("/report-cards/generate", op("post", "generateReportCard", "Generate a report card", ["report-cards"], request="GenerateReportCardRequest", response="ReportCard", response_code="201")),
            ("/transcripts/{student_id}", op("get", "getTranscript", "Get a student transcript", ["transcripts"], params=["$ref: '#/components/parameters/TenantId'"], response="Transcript")),
            ("/report-templates", op("get", "listReportTemplates", "List report templates", ["templates"], response="ReportTemplateList")),
            ("/report-templates", op("post", "createReportTemplate", "Create a report template", ["templates"], request="CreateReportTemplate", response="ReportTemplate", response_code="201")),
        ],
        "schemas": [
            schema("ReportCard", "type: object\nrequired: [id, tenant_id, student_id, term_id]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  term_id: { type: string, format: uuid }\n  file_url: { type: [string, 'null'] }\n  generated_at: { type: string, format: date-time }"),
            schema("GenerateReportCardRequest", "type: object\nrequired: [student_id, term_id]\nproperties:\n  student_id: { type: string, format: uuid }\n  term_id: { type: string, format: uuid }"),
            schema("Transcript", "type: object\nrequired: [id, tenant_id, student_id]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  file_url: { type: [string, 'null'] }\n  entries: { type: array, items: { type: object, properties: { subject_id: { type: string, format: uuid }, subject_name: { type: string }, score: { type: number }, grade: { type: string } } } }"),
            schema("ReportTemplate", "type: object\nrequired: [id, tenant_id, name]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  layout: { type: [string, 'null'] }\n  is_default: { type: boolean }"),
            schema("CreateReportTemplate", "type: object\nrequired: [name]\nproperties:\n  name: { type: string }\n  layout: { type: [string, 'null'] }\n  is_default: { type: boolean, default: false }"),
            schema("ReportCardList", res("ReportCard", is_array=True)),
            schema("ReportTemplateList", res("ReportTemplate", is_array=True)),
        ],
    },
    {
        "key": "fees",
        "title": "AuraEDU Fees Service API",
        "description": "Fee structures, invoices, balances, and receipts (spec \\u00a77). Owned by lane L2 (EP-16). Feature flag: fees.",
        "tags": ["fee-structures", "invoices", "balances", "receipts"],
        "paths": [
            ("/fee-structures", op("get", "listFeeStructures", "List fee structures", ["fee-structures"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="FeeStructureList")),
            ("/fee-structures", op("post", "createFeeStructure", "Create a fee structure", ["fee-structures"], request="CreateFeeStructure", response="FeeStructure", response_code="201")),
            ("/invoices", op("get", "listInvoices", "List invoices", ["invoices"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="InvoiceList")),
            ("/invoices", op("post", "createInvoice", "Create an invoice", ["invoices"], request="CreateInvoice", response="Invoice", response_code="201")),
            ("/invoices/{invoice_id}", op("get", "getInvoice", "Get an invoice", ["invoices"], params=["$ref: '#/components/parameters/TenantId'"], response="Invoice")),
            ("/balances/{student_id}", op("get", "getBalance", "Get a student balance", ["balances"], params=["$ref: '#/components/parameters/TenantId'"], response="Balance")),
            ("/receipts/{receipt_id}", op("get", "getReceipt", "Get a receipt", ["receipts"], params=["$ref: '#/components/parameters/TenantId'"], response="Receipt")),
        ],
        "schemas": [
            schema("FeeStructure", "type: object\nrequired: [id, tenant_id, name]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  class_ids: { type: array, items: { type: string, format: uuid } }\n  amount: { type: number, minimum: 0 }\n  term_id: { type: [string, 'null'], format: uuid }"),
            schema("CreateFeeStructure", "type: object\nrequired: [name, amount]\nproperties:\n  name: { type: string }\n  class_ids: { type: array, items: { type: string, format: uuid } }\n  amount: { type: number, minimum: 0 }\n  term_id: { type: [string, 'null'], format: uuid }"),
            schema("Invoice", "type: object\nrequired: [id, tenant_id, student_id, amount_due]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  amount_due: { type: number, minimum: 0 }\n  amount_paid: { type: number, minimum: 0, default: 0 }\n  due_date: { type: [string, 'null'], format: date }\n  status: { type: string, enum: [pending, partial, paid, overdue] }"),
            schema("CreateInvoice", "type: object\nrequired: [student_id, amount_due]\nproperties:\n  student_id: { type: string, format: uuid }\n  amount_due: { type: number, minimum: 0 }\n  due_date: { type: [string, 'null'], format: date }"),
            schema("Balance", "type: object\nrequired: [student_id, total_due, total_paid]\nproperties:\n  student_id: { type: string, format: uuid }\n  total_due: { type: number, minimum: 0 }\n  total_paid: { type: number, minimum: 0 }\n  outstanding: { type: number }"),
            schema("Receipt", "type: object\nrequired: [id, tenant_id, invoice_id, amount]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  invoice_id: { type: string, format: uuid }\n  amount: { type: number, minimum: 0 }\n  payment_id: { type: [string, 'null'], format: uuid }\n  issued_at: { type: string, format: date-time }"),
            schema("FeeStructureList", res("FeeStructure", is_array=True)),
            schema("InvoiceList", res("Invoice", is_array=True)),
        ],
    },
    {
        "key": "payment",
        "title": "AuraEDU Payment Service API",
        "description": "Payment gateway integrations, transactions, and webhooks (spec \\u00a77). Owned by lane L2 (EP-17). Feature flag: online_payments.",
        "tags": ["payments", "transactions", "webhooks"],
        "paths": [
            ("/payments/initiate", op("post", "initiatePayment", "Initiate a payment", ["payments"], request="InitiatePaymentRequest", response="Payment", response_code="201")),
            ("/payments/{payment_id}", op("get", "getPayment", "Get payment status", ["payments"], params=["$ref: '#/components/parameters/TenantId'"], response="Payment")),
            ("/transactions", op("get", "listTransactions", "List transactions", ["transactions"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="TransactionList")),
            ("/webhooks/{provider}", op("post", "receiveWebhook", "Receive provider webhook", ["webhooks"], params=["$ref: '#/components/parameters/TenantId'"], request="WebhookPayload", response="WebhookEvent", response_code="201", no_auth=True)),
        ],
        "schemas": [
            schema("InitiatePaymentRequest", "type: object\nrequired: [invoice_id, amount, gateway]\nproperties:\n  invoice_id: { type: string, format: uuid }\n  amount: { type: number, minimum: 0 }\n  gateway: { type: string, enum: [paystack, flutterwave] }\n  callback_url: { type: [string, 'null'] }"),
            schema("Payment", "type: object\nrequired: [id, tenant_id, invoice_id, amount, status]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  invoice_id: { type: string, format: uuid }\n  amount: { type: number, minimum: 0 }\n  gateway: { type: string }\n  status: { type: string, enum: [pending, success, failed] }\n  reference: { type: [string, 'null'] }"),
            schema("Transaction", "type: object\nrequired: [id, tenant_id, payment_id, amount]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  payment_id: { type: string, format: uuid }\n  amount: { type: number, minimum: 0 }\n  gateway_reference: { type: [string, 'null'] }\n  recorded_at: { type: string, format: date-time }"),
            schema("WebhookPayload", "type: object\nadditionalProperties: true\nproperties:\n  event: { type: string }\n  data: { type: object, additionalProperties: true }"),
            schema("WebhookEvent", "type: object\nrequired: [id, provider, event]\nproperties:\n  id: { type: string, format: uuid }\n  provider: { type: string }\n  event: { type: string }\n  processed: { type: boolean }"),
            schema("TransactionList", res("Transaction", is_array=True)),
        ],
    },
    {
        "key": "notification",
        "title": "AuraEDU Notification Service API",
        "description": "Email, SMS, WhatsApp, in-app, and announcements (spec \\u00a77). Owned by lane L2 (EP-18). Feature flags: email_notifications, sms_notifications, whatsapp_notifications, announcements.",
        "tags": ["messages", "templates", "subscriptions"],
        "paths": [
            ("/messages", op("get", "listMessages", "List messages", ["messages"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="MessageList")),
            ("/messages/send", op("post", "sendMessage", "Send a message", ["messages"], request="SendMessageRequest", response="Message", response_code="201")),
            ("/templates", op("get", "listTemplates", "List templates", ["templates"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="TemplateList")),
            ("/templates", op("post", "createTemplate", "Create a template", ["templates"], request="CreateTemplate", response="Template", response_code="201")),
            ("/subscriptions", op("get", "listSubscriptions", "List subscriptions", ["subscriptions"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="SubscriptionList")),
            ("/subscriptions", op("post", "createSubscription", "Create a subscription", ["subscriptions"], request="CreateSubscription", response="Subscription", response_code="201")),
        ],
        "schemas": [
            schema("Message", "type: object\nrequired: [id, tenant_id, channel, recipient_id, status]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  channel: { type: string, enum: [email, sms, whatsapp, in_app] }\n  recipient_id: { type: string, format: uuid }\n  subject: { type: [string, 'null'] }\n  body: { type: [string, 'null'] }\n  status: { type: string, enum: [queued, sent, delivered, failed] }"),
            schema("SendMessageRequest", "type: object\nrequired: [channel, recipient_id]\nproperties:\n  channel: { type: string, enum: [email, sms, whatsapp, in_app] }\n  recipient_id: { type: string, format: uuid }\n  subject: { type: [string, 'null'] }\n  body: { type: [string, 'null'] }\n  template_id: { type: [string, 'null'], format: uuid }\n  variables: { type: [object, 'null'], additionalProperties: true }"),
            schema("Template", "type: object\nrequired: [id, tenant_id, name, channel]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  channel: { type: string, enum: [email, sms, whatsapp, in_app] }\n  subject_template: { type: [string, 'null'] }\n  body_template: { type: string }"),
            schema("CreateTemplate", "type: object\nrequired: [name, channel, body_template]\nproperties:\n  name: { type: string }\n  channel: { type: string, enum: [email, sms, whatsapp, in_app] }\n  subject_template: { type: [string, 'null'] }\n  body_template: { type: string }"),
            schema("Subscription", "type: object\nrequired: [id, tenant_id, user_id, channel]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  user_id: { type: string, format: uuid }\n  channel: { type: string, enum: [email, sms, whatsapp, in_app] }\n  is_enabled: { type: boolean }"),
            schema("CreateSubscription", "type: object\nrequired: [user_id, channel]\nproperties:\n  user_id: { type: string, format: uuid }\n  channel: { type: string, enum: [email, sms, whatsapp, in_app] }\n  is_enabled: { type: boolean, default: true }"),
            schema("MessageList", res("Message", is_array=True)),
            schema("TemplateList", res("Template", is_array=True)),
            schema("SubscriptionList", res("Subscription", is_array=True)),
        ],
    },
    {
        "key": "website",
        "title": "AuraEDU Website Service API",
        "description": "Public site content, pages, and menus (spec \\u00a77). Owned by lane L2 (EP-19). Feature flag: public_website.",
        "tags": ["pages", "sections", "menus"],
        "paths": [
            ("/pages", op("get", "listPages", "List pages", ["pages"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="PageList")),
            ("/pages", op("post", "createPage", "Create a page", ["pages"], request="CreatePage", response="Page", response_code="201")),
            ("/pages/{slug}", op("get", "getPageBySlug", "Get a public page by slug", ["pages"], params=["$ref: '#/components/parameters/TenantId'"], response="Page")),
            ("/menus", op("get", "listMenus", "List menus", ["menus"], response="MenuList")),
            ("/menus", op("post", "createMenu", "Create a menu", ["menus"], request="CreateMenu", response="Menu", response_code="201")),
        ],
        "schemas": [
            schema("Page", "type: object\nrequired: [id, tenant_id, slug, title]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  slug: { type: string }\n  title: { type: string }\n  sections: { type: array, items: { $ref: '#/components/schemas/Section' } }\n  is_published: { type: boolean }\n  published_at: { type: [string, 'null'], format: date-time }"),
            schema("CreatePage", "type: object\nrequired: [slug, title]\nproperties:\n  slug: { type: string }\n  title: { type: string }\n  sections: { type: array, items: { $ref: '#/components/schemas/Section' } }\n  is_published: { type: boolean, default: false }"),
            schema("Section", "type: object\nrequired: [type]\nproperties:\n  type: { type: string, enum: [hero, text, gallery, call_to_action] }\n  title: { type: [string, 'null'] }\n  content: { type: [string, 'null'] }\n  media: { type: array, items: { type: string } }"),
            schema("Menu", "type: object\nrequired: [id, tenant_id, name]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  items: { type: array, items: { type: object, properties: { label: { type: string }, url: { type: string }, page_id: { type: [string, 'null'], format: uuid } } } }"),
            schema("CreateMenu", "type: object\nrequired: [name]\nproperties:\n  name: { type: string }\n  items: { type: array, items: { type: object, properties: { label: { type: string }, url: { type: string }, page_id: { type: [string, 'null'], format: uuid } } } }"),
            schema("PageList", res("Page", is_array=True)),
            schema("MenuList", res("Menu", is_array=True)),
        ],
    },
    {
        "key": "file",
        "title": "AuraEDU File Service API",
        "description": "Uploads, documents, images, and certificates backed by Cloudinary (spec \\u00a77). Owned by lane L2 (EP-20).",
        "tags": ["uploads", "files"],
        "paths": [
            ("/uploads/signed", op("post", "requestSignedUpload", "Request a signed Cloudinary upload", ["uploads"], request="SignedUploadRequest", response="SignedUploadResponse", response_code="201")),
            ("/files", op("get", "listFiles", "List files", ["files"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="FileList")),
            ("/files/{file_id}", op("get", "getFile", "Get a file", ["files"], params=["$ref: '#/components/parameters/TenantId'"], response="File")),
            ("/files/{file_id}", op("delete", "deleteFile", "Delete a file", ["files"], params=["$ref: '#/components/parameters/TenantId'"], response_code="204")),
        ],
        "schemas": [
            schema("SignedUploadRequest", "type: object\nrequired: [folder, file_name]\nproperties:\n  folder: { type: string, example: upshs/documents }\n  file_name: { type: string }\n  resource_type: { type: string, enum: [image, raw, video], default: image }"),
            schema("SignedUploadResponse", "type: object\nrequired: [signature, timestamp, api_key, folder]\nproperties:\n  signature: { type: string }\n  timestamp: { type: integer }\n  api_key: { type: string }\n  folder: { type: string }\n  cloud_name: { type: string }\n  upload_url: { type: string }"),
            schema("File", "type: object\nrequired: [id, tenant_id, public_id, secure_url]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  public_id: { type: string }\n  secure_url: { type: string }\n  resource_type: { type: string, enum: [image, raw, video] }\n  folder: { type: string }\n  uploaded_by: { type: [string, 'null'], format: uuid }"),
            schema("FileList", res("File", is_array=True)),
        ],
    },
    {
        "key": "analytics",
        "title": "AuraEDU Analytics Service API",
        "description": "KPIs, dashboards, and projections (spec \\u00a77). Owned by lane L2 (EP-21). Feature flag: analytics.",
        "tags": ["kpis", "projections"],
        "paths": [
            ("/kpis", op("get", "listKpis", "List KPI snapshots", ["kpis"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="KpiSnapshotList")),
            ("/projections", op("get", "listProjections", "List projections", ["projections"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="ProjectionList")),
        ],
        "schemas": [
            schema("KpiSnapshot", "type: object\nrequired: [id, tenant_id, metric_key, value]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  metric_key: { type: string }\n  value: { type: number }\n  recorded_at: { type: string, format: date-time }"),
            schema("Projection", "type: object\nrequired: [id, tenant_id, metric_key, projected_value]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  metric_key: { type: string }\n  projected_value: { type: number }\n  horizon: { type: string }\n  generated_at: { type: string, format: date-time }"),
            schema("KpiSnapshotList", res("KpiSnapshot", is_array=True)),
            schema("ProjectionList", res("Projection", is_array=True)),
        ],
    },
    {
        "key": "billing",
        "title": "AuraEDU Billing Service API",
        "description": "SaaS plans, subscriptions, and invoices (spec \\u00a77). Owned by lane L2 (EP-22). Feature flag: billing.",
        "tags": ["plans", "subscriptions", "invoices"],
        "paths": [
            ("/plans", op("get", "listPlans", "List SaaS plans", ["plans"], response="PlanList")),
            ("/subscriptions", op("get", "listSubscriptions", "List tenant subscriptions", ["subscriptions"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="SubscriptionList")),
            ("/subscriptions", op("post", "createSubscription", "Create or update subscription", ["subscriptions"], request="CreateSubscription", response="Subscription", response_code="201")),
            ("/saas-invoices", op("get", "listSaasInvoices", "List SaaS invoices", ["invoices"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="SaasInvoiceList")),
        ],
        "schemas": [
            schema("Plan", "type: object\nrequired: [id, key, name]\nproperties:\n  id: { type: string, format: uuid }\n  key: { type: string }\n  name: { type: string }\n  features: { type: array, items: { type: string } }\n  price_monthly: { type: [number, 'null'], minimum: 0 }"),
            schema("Subscription", "type: object\nrequired: [id, tenant_id, plan_key, status]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  plan_key: { type: string }\n  status: { type: string, enum: [active, canceled, past_due] }\n  current_period_start: { type: string, format: date-time }\n  current_period_end: { type: string, format: date-time }"),
            schema("CreateSubscription", "type: object\nrequired: [tenant_id, plan_key]\nproperties:\n  tenant_id: { type: string, format: uuid }\n  plan_key: { type: string }\n  billing_email: { type: [string, 'null'], format: email }"),
            schema("SaasInvoice", "type: object\nrequired: [id, tenant_id, amount, status]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  amount: { type: number, minimum: 0 }\n  status: { type: string, enum: [draft, open, paid, void] }\n  due_date: { type: [string, 'null'], format: date }"),
            schema("PlanList", res("Plan", is_array=True)),
            schema("SubscriptionList", res("Subscription", is_array=True)),
            schema("SaasInvoiceList", res("SaasInvoice", is_array=True)),
        ],
    },
    {
        "key": "cbt",
        "title": "AuraEDU CBT Service API",
        "description": "Computer-based / online exams: question banks, sessions, submissions, and auto-grading (spec \\u00a77). Owned by lane L2 (EP-24). Feature flag: cbt_exams.",
        "tags": ["exams", "question-banks", "submissions"],
        "paths": [
            ("/exams", op("get", "listExams", "List exams", ["exams"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="ExamList")),
            ("/exams", op("post", "createExam", "Create an exam", ["exams"], request="CreateExam", response="Exam", response_code="201")),
            ("/exams/{exam_id}", op("get", "getExam", "Get an exam", ["exams"], params=["$ref: '#/components/parameters/TenantId'"], response="Exam")),
            ("/exams/{exam_id}/sessions", op("post", "startExamSession", "Start an exam session", ["exams"], params=["$ref: '#/components/parameters/TenantId'"], response="ExamSession", response_code="201")),
            ("/submissions", op("post", "submitExam", "Submit an exam", ["submissions"], request="CreateSubmission", response="Submission", response_code="201")),
            ("/question-banks", op("get", "listQuestionBanks", "List question banks", ["question-banks"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="QuestionBankList")),
            ("/question-banks", op("post", "createQuestionBank", "Create a question bank", ["question-banks"], request="CreateQuestionBank", response="QuestionBank", response_code="201")),
        ],
        "schemas": [
            schema("Exam", "type: object\nrequired: [id, tenant_id, title, subject_id]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  title: { type: string }\n  subject_id: { type: string, format: uuid }\n  duration_minutes: { type: integer, minimum: 1 }\n  question_count: { type: integer, minimum: 0 }\n  starts_at: { type: [string, 'null'], format: date-time }\n  ends_at: { type: [string, 'null'], format: date-time }"),
            schema("CreateExam", "type: object\nrequired: [title, subject_id]\nproperties:\n  title: { type: string }\n  subject_id: { type: string, format: uuid }\n  duration_minutes: { type: integer, minimum: 1 }\n  question_ids: { type: array, items: { type: string, format: uuid } }\n  starts_at: { type: [string, 'null'], format: date-time }\n  ends_at: { type: [string, 'null'], format: date-time }"),
            schema("ExamSession", "type: object\nrequired: [id, exam_id, student_id, started_at]\nproperties:\n  id: { type: string, format: uuid }\n  exam_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  started_at: { type: string, format: date-time }\n  expires_at: { type: string, format: date-time }"),
            schema("Submission", "type: object\nrequired: [id, exam_id, student_id, answers]\nproperties:\n  id: { type: string, format: uuid }\n  exam_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  answers: { type: array, items: { type: object, properties: { question_id: { type: string, format: uuid }, selected_option: { type: [string, 'null'] }, text_answer: { type: [string, 'null'] } } } }\n  score: { type: [number, 'null'], minimum: 0 }\n  max_score: { type: [number, 'null'], minimum: 0 }\n  submitted_at: { type: string, format: date-time }"),
            schema("CreateSubmission", "type: object\nrequired: [exam_id, student_id, answers]\nproperties:\n  exam_id: { type: string, format: uuid }\n  student_id: { type: string, format: uuid }\n  answers: { type: array, items: { type: object, properties: { question_id: { type: string, format: uuid }, selected_option: { type: [string, 'null'] }, text_answer: { type: [string, 'null'] } } } }"),
            schema("QuestionBank", "type: object\nrequired: [id, tenant_id, name]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  name: { type: string }\n  subject_id: { type: [string, 'null'], format: uuid }\n  question_count: { type: integer, minimum: 0 }"),
            schema("CreateQuestionBank", "type: object\nrequired: [name]\nproperties:\n  name: { type: string }\n  subject_id: { type: [string, 'null'], format: uuid }"),
            schema("ExamList", res("Exam", is_array=True)),
            schema("QuestionBankList", res("QuestionBank", is_array=True)),
        ],
    },
    {
        "key": "audit",
        "title": "AuraEDU Audit Service API",
        "description": "Immutable audit log sink for compliance (spec \\u00a77). Owned by lane L2 (EP-23).",
        "tags": ["audit-logs"],
        "paths": [
            ("/audit-logs", op("get", "listAuditLogs", "List audit logs", ["audit-logs"], params=["$ref: '#/components/parameters/Limit'", "$ref: '#/components/parameters/Cursor'"], response="AuditLogList")),
        ],
        "schemas": [
            schema("AuditLog", "type: object\nrequired: [id, tenant_id, event_type, actor_id, occurred_at]\nproperties:\n  id: { type: string, format: uuid }\n  tenant_id: { type: string, format: uuid }\n  event_type: { type: string }\n  actor_id: { type: [string, 'null'], format: uuid }\n  resource_type: { type: [string, 'null'] }\n  resource_id: { type: [string, 'null'] }\n  metadata: { type: [object, 'null'], additionalProperties: true }\n  occurred_at: { type: string, format: date-time }"),
            schema("AuditLogList", res("AuditLog", is_array=True)),
        ],
    },
]


def render_paths(spec: dict) -> str:
    """Group operations by path and render the YAML block."""
    by_path: dict[str, list[str]] = {}
    for path, operation_yaml in spec["paths"]:
        by_path.setdefault(path, []).append(operation_yaml)
    lines = []
    for path, ops in by_path.items():
        lines.append(f"  {path}:")
        for op_text in ops:
            lines.append(op_text)
    return "\n".join(lines)


def write_openapi_specs() -> None:
    OPENAPI_DIR.mkdir(parents=True, exist_ok=True)
    for spec in OPENAPI_SPECS:
        dest = OPENAPI_DIR / f"{spec['key']}.v1.yaml"
        if dest.exists():
            print(f"skip existing {dest}")
            continue
        paths_yaml = render_paths(spec)
        schemas_yaml = "\n".join(spec["schemas"])
        content = make_openapi(
            spec["key"],
            spec["title"],
            spec["description"],
            spec["tags"],
            paths_yaml,
            schemas_yaml,
        )
        dest.write_text(content, encoding="utf-8")
        print(f"wrote {dest}")


# ---------------------------------------------------------------------------
# CloudEvents seed definitions
# ---------------------------------------------------------------------------
EVENTS: list[dict] = [
    {
        "type": "tenant.feature_enabled",
        "source": "tenant-service",
        "description": "Emitted when a feature flag is enabled for a tenant.",
        "data_required": ["feature_key"],
        "data_props": {
            "feature_key": {"type": "string"},
            "plan_required": {"type": ["string", "null"]},
            "config": {"type": ["object", "null"], "additionalProperties": True},
        },
    },
    {
        "type": "tenant.feature_disabled",
        "source": "tenant-service",
        "description": "Emitted when a feature flag is disabled for a tenant.",
        "data_required": ["feature_key"],
        "data_props": {
            "feature_key": {"type": "string"},
        },
    },
    {
        "type": "user.role_changed",
        "source": "identity-service",
        "description": "Emitted when a user's role or permissions change.",
        "data_required": ["user_id", "role"],
        "data_props": {
            "user_id": {"type": "string", "format": "uuid"},
            "role": {"type": "string"},
            "previous_role": {"type": ["string", "null"]},
            "permissions": {"type": "array", "items": {"type": "string"}},
        },
    },
    {
        "type": "student.enrolled",
        "source": "student-service",
        "description": "Emitted when a student is enrolled in a class.",
        "data_required": ["student_id", "class_id", "academic_year_id"],
        "data_props": {
            "student_id": {"type": "string", "format": "uuid"},
            "class_id": {"type": "string", "format": "uuid"},
            "academic_year_id": {"type": "string", "format": "uuid"},
            "enrollment_date": {"type": "string", "format": "date"},
        },
    },
    {
        "type": "student.updated",
        "source": "student-service",
        "description": "Emitted when a student record is updated.",
        "data_required": ["student_id"],
        "data_props": {
            "student_id": {"type": "string", "format": "uuid"},
            "changed_fields": {"type": "array", "items": {"type": "string"}},
        },
    },
    {
        "type": "staff.created",
        "source": "staff-service",
        "description": "Emitted when a staff record is created.",
        "data_required": ["staff_id", "staff_type"],
        "data_props": {
            "staff_id": {"type": "string", "format": "uuid"},
            "staff_type": {"type": "string", "enum": ["teacher", "non_teaching"]},
            "name": {"type": "string"},
        },
    },
    {
        "type": "staff.assigned",
        "source": "staff-service",
        "description": "Emitted when a staff member is assigned to a class/subject.",
        "data_required": ["staff_id"],
        "data_props": {
            "staff_id": {"type": "string", "format": "uuid"},
            "class_id": {"type": ["string", "null"], "format": "uuid"},
            "subject_id": {"type": ["string", "null"], "format": "uuid"},
            "role": {"type": ["string", "null"]},
        },
    },
    {
        "type": "academic.year_created",
        "source": "academic-service",
        "description": "Emitted when an academic year is created.",
        "data_required": ["year_id", "name"],
        "data_props": {
            "year_id": {"type": "string", "format": "uuid"},
            "name": {"type": "string"},
            "start_date": {"type": "string", "format": "date"},
            "end_date": {"type": "string", "format": "date"},
        },
    },
    {
        "type": "academic.class_created",
        "source": "academic-service",
        "description": "Emitted when a class is created.",
        "data_required": ["class_id", "name", "academic_year_id"],
        "data_props": {
            "class_id": {"type": "string", "format": "uuid"},
            "name": {"type": "string"},
            "academic_year_id": {"type": "string", "format": "uuid"},
        },
    },
    {
        "type": "academic.subject_created",
        "source": "academic-service",
        "description": "Emitted when a subject is created.",
        "data_required": ["subject_id", "name"],
        "data_props": {
            "subject_id": {"type": "string", "format": "uuid"},
            "name": {"type": "string"},
            "code": {"type": ["string", "null"]},
        },
    },
    {
        "type": "attendance.marked",
        "source": "attendance-service",
        "description": "Emitted when attendance is recorded.",
        "data_required": ["student_id", "date", "status"],
        "data_props": {
            "student_id": {"type": "string", "format": "uuid"},
            "class_id": {"type": ["string", "null"], "format": "uuid"},
            "subject_id": {"type": ["string", "null"], "format": "uuid"},
            "date": {"type": "string", "format": "date"},
            "status": {"type": "string", "enum": ["present", "absent", "late", "excused"]},
            "recorded_by": {"type": "string", "format": "uuid"},
        },
    },
    {
        "type": "assignment.published",
        "source": "assessment-service",
        "description": "Emitted when an assignment is published.",
        "data_required": ["assignment_id", "subject_id"],
        "data_props": {
            "assignment_id": {"type": "string", "format": "uuid"},
            "subject_id": {"type": "string", "format": "uuid"},
            "class_ids": {"type": "array", "items": {"type": "string", "format": "uuid"}},
            "due_date": {"type": ["string", "null"], "format": "date-time"},
        },
    },
    {
        "type": "report.published",
        "source": "report-service",
        "description": "Emitted when a report card is published.",
        "data_required": ["report_card_id", "student_id", "term_id"],
        "data_props": {
            "report_card_id": {"type": "string", "format": "uuid"},
            "student_id": {"type": "string", "format": "uuid"},
            "term_id": {"type": "string", "format": "uuid"},
            "file_url": {"type": ["string", "null"]},
        },
    },
    {
        "type": "invoice.created",
        "source": "fees-service",
        "description": "Emitted when an invoice is created.",
        "data_required": ["invoice_id", "student_id", "amount_due"],
        "data_props": {
            "invoice_id": {"type": "string", "format": "uuid"},
            "student_id": {"type": "string", "format": "uuid"},
            "amount_due": {"type": "number", "minimum": 0},
            "due_date": {"type": ["string", "null"], "format": "date"},
        },
    },
    {
        "type": "fee.assigned",
        "source": "fees-service",
        "description": "Emitted when a fee structure is assigned to a class.",
        "data_required": ["fee_structure_id"],
        "data_props": {
            "fee_structure_id": {"type": "string", "format": "uuid"},
            "class_id": {"type": ["string", "null"], "format": "uuid"},
            "amount": {"type": ["number", "null"], "minimum": 0},
        },
    },
    {
        "type": "payment.received",
        "source": "payment-service",
        "description": "Emitted when a payment is successfully received.",
        "data_required": ["payment_id", "invoice_id", "amount"],
        "data_props": {
            "payment_id": {"type": "string", "format": "uuid"},
            "invoice_id": {"type": "string", "format": "uuid"},
            "amount": {"type": "number", "minimum": 0},
            "gateway": {"type": "string"},
        },
    },
    {
        "type": "payment.failed",
        "source": "payment-service",
        "description": "Emitted when a payment fails.",
        "data_required": ["payment_id"],
        "data_props": {
            "payment_id": {"type": "string", "format": "uuid"},
            "invoice_id": {"type": ["string", "null"], "format": "uuid"},
            "reason": {"type": ["string", "null"]},
        },
    },
    {
        "type": "notification.sent",
        "source": "notification-service",
        "description": "Emitted when a notification is sent.",
        "data_required": ["message_id", "channel", "recipient_id"],
        "data_props": {
            "message_id": {"type": "string", "format": "uuid"},
            "channel": {"type": "string", "enum": ["email", "sms", "whatsapp", "in_app"]},
            "recipient_id": {"type": "string", "format": "uuid"},
        },
    },
    {
        "type": "notification.failed",
        "source": "notification-service",
        "description": "Emitted when a notification fails to send.",
        "data_required": ["message_id", "channel"],
        "data_props": {
            "message_id": {"type": "string", "format": "uuid"},
            "channel": {"type": "string", "enum": ["email", "sms", "whatsapp", "in_app"]},
            "reason": {"type": ["string", "null"]},
        },
    },
    {
        "type": "website.page_published",
        "source": "website-service",
        "description": "Emitted when a website page is published.",
        "data_required": ["page_id", "slug"],
        "data_props": {
            "page_id": {"type": "string", "format": "uuid"},
            "slug": {"type": "string"},
            "title": {"type": ["string", "null"]},
        },
    },
    {
        "type": "file.uploaded",
        "source": "file-service",
        "description": "Emitted when a file is uploaded to Cloudinary.",
        "data_required": ["file_id", "public_id"],
        "data_props": {
            "file_id": {"type": "string", "format": "uuid"},
            "public_id": {"type": "string"},
            "secure_url": {"type": ["string", "null"]},
            "folder": {"type": ["string", "null"]},
        },
    },
    {
        "type": "analytics.metric_updated",
        "source": "analytics-service",
        "description": "Emitted when an analytics metric is updated.",
        "data_required": ["metric_key", "value"],
        "data_props": {
            "metric_key": {"type": "string"},
            "value": {"type": "number"},
            "timestamp": {"type": "string", "format": "date-time"},
        },
    },
    {
        "type": "ai.recommendation_generated",
        "source": "ai-recommendation-service",
        "description": "Emitted when an AI learning recommendation is generated.",
        "data_required": ["student_id", "recommendation_type"],
        "data_props": {
            "student_id": {"type": "string", "format": "uuid"},
            "recommendation_type": {"type": "string"},
            "confidence": {"type": ["number", "null"], "minimum": 0, "maximum": 1},
            "explanation": {"type": ["string", "null"]},
        },
    },
    {
        "type": "billing.subscription_changed",
        "source": "billing-service",
        "description": "Emitted when a tenant subscription changes.",
        "data_required": ["tenant_id", "plan_key", "status"],
        "data_props": {
            "tenant_id": {"type": "string", "format": "uuid"},
            "plan_key": {"type": "string"},
            "status": {"type": "string", "enum": ["active", "canceled", "past_due"]},
        },
    },
    {
        "type": "billing.plan_upgraded",
        "source": "billing-service",
        "description": "Emitted when a tenant upgrades their plan.",
        "data_required": ["tenant_id", "plan"],
        "data_props": {
            "tenant_id": {"type": "string", "format": "uuid"},
            "plan": {"type": "string"},
            "previous_plan": {"type": ["string", "null"]},
        },
    },
    {
        "type": "cbt.exam_submitted",
        "source": "cbt-service",
        "description": "Emitted when a CBT exam is submitted.",
        "data_required": ["exam_id", "student_id", "submission_id"],
        "data_props": {
            "exam_id": {"type": "string", "format": "uuid"},
            "student_id": {"type": "string", "format": "uuid"},
            "submission_id": {"type": "string", "format": "uuid"},
            "submitted_at": {"type": "string", "format": "date-time"},
        },
    },
    {
        "type": "cbt.graded",
        "source": "cbt-service",
        "description": "Emitted when a CBT submission is graded.",
        "data_required": ["submission_id", "score"],
        "data_props": {
            "submission_id": {"type": "string", "format": "uuid"},
            "score": {"type": "number", "minimum": 0},
            "max_score": {"type": ["number", "null"], "minimum": 0},
        },
    },
]


def write_events() -> None:
    EVENTS_DIR.mkdir(parents=True, exist_ok=True)
    for evt in EVENTS:
        dest = EVENTS_DIR / f"{evt['type']}.v1.json"
        if dest.exists():
            print(f"skip existing {dest}")
            continue
        schema = {
            "$schema": "https://json-schema.org/draft/2020-12/schema",
            "$id": f"https://contracts.auraedu.dev/events/{evt['type']}.v1.json",
            "title": evt["type"],
            "description": evt["description"],
            "type": "object",
            "required": ["specversion", "type", "source", "id", "time", "tenant_id", "data"],
            "properties": {
                "specversion": {"const": "1.0"},
                "type": {"const": evt["type"]},
                "source": {"const": evt["source"]},
                "subject": {"type": "string"},
                "id": {"type": "string"},
                "time": {"type": "string", "format": "date-time"},
                "tenant_id": {"type": "string"},
                "datacontenttype": {"const": "application/json"},
                "data": {
                    "type": "object",
                    "required": evt["data_required"],
                    "properties": evt["data_props"],
                },
            },
        }
        dest.write_text(json.dumps(schema, indent=2) + "\n", encoding="utf-8")
        print(f"wrote {dest}")


if __name__ == "__main__":
    write_openapi_specs()
    write_events()
    print("Done seeding contracts.")
