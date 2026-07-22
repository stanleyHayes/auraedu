import assert from "node:assert/strict";
import test from "node:test";

import { summarizeTeacherAnalytics } from "../lib/teacher-analytics.ts";

void test("teacher analytics uses sample-weighted scores and real attendance totals", () => {
  const summary = summarizeTeacherAnalytics({
    "assessments.avg_percentage": [
      {
        metric_name: "assessments.avg_percentage",
        bucket_date: "2026-07-01",
        value: 50,
        sample_count: 1,
      },
      {
        metric_name: "assessments.avg_percentage",
        bucket_date: "2026-07-01",
        value: 80,
        sample_count: 3,
      },
      {
        metric_name: "assessments.avg_percentage",
        bucket_date: "2026-07-02",
        value: 90,
        sample_count: 2,
      },
    ],
    "assessments.count": [
      { metric_name: "assessments.count", bucket_date: "2026-07-01", value: 6 },
    ],
    "attendance.present": [
      { metric_name: "attendance.present", bucket_date: "2026-07-01", value: 8 },
    ],
    "attendance.absent": [
      { metric_name: "attendance.absent", bucket_date: "2026-07-01", value: 2 },
    ],
    "students.count": [{ metric_name: "students.count", bucket_date: "2026-07-01", value: 3 }],
    "reports.count": [{ metric_name: "reports.count", bucket_date: "2026-07-01", value: 4 }],
  });

  assert.equal(summary.averagePercentage, 470 / 6);
  assert.equal(summary.attendanceRate, 80);
  assert.equal(summary.scoreRecords, 6);
  assert.equal(summary.newEnrolments, 3);
  assert.equal(summary.reportsPublished, 4);
  assert.deepEqual(summary.dailyPerformance, [
    { date: "2026-07-01", value: 72.5 },
    { date: "2026-07-02", value: 90 },
  ]);
  assert.equal(summary.improvement, 17.5);
});

void test("teacher analytics reports unavailable rates instead of fabricated zeroes", () => {
  const summary = summarizeTeacherAnalytics({});
  assert.equal(summary.averagePercentage, null);
  assert.equal(summary.attendanceRate, null);
  assert.equal(summary.improvement, null);
  assert.deepEqual(summary.dailyPerformance, []);
});
