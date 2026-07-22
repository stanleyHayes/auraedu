export interface TeacherMetric {
  metric_name: string;
  bucket_date: string;
  value: number;
  sample_count?: number | null;
}

export interface TeacherAnalyticsSummary {
  averagePercentage: number | null;
  attendanceRate: number | null;
  scoreRecords: number;
  newEnrolments: number;
  reportsPublished: number;
  improvement: number | null;
  dailyPerformance: { date: string; value: number }[];
}

function total(rows: TeacherMetric[]) {
  return rows.reduce((sum, row) => sum + row.value, 0);
}

function weightedAverage(rows: TeacherMetric[]) {
  const aggregate = rows.reduce(
    (result, row) => {
      const samples = Math.max(1, row.sample_count ?? 1);
      result.weighted += row.value * samples;
      result.samples += samples;
      return result;
    },
    { weighted: 0, samples: 0 },
  );
  return aggregate.samples > 0 ? aggregate.weighted / aggregate.samples : null;
}

export function summarizeTeacherAnalytics(
  metrics: Record<string, TeacherMetric[]>,
): TeacherAnalyticsSummary {
  const scores = metrics["assessments.avg_percentage"] ?? [];
  const byDate = new Map<string, TeacherMetric[]>();
  for (const row of scores) {
    const values = byDate.get(row.bucket_date) ?? [];
    values.push(row);
    byDate.set(row.bucket_date, values);
  }
  const dailyPerformance = [...byDate.entries()]
    .map(([date, rows]) => ({ date, value: weightedAverage(rows) ?? 0 }))
    .sort((left, right) => left.date.localeCompare(right.date));

  const midpoint = Math.ceil(dailyPerformance.length / 2);
  const earlier = dailyPerformance.slice(0, midpoint);
  const recent = dailyPerformance.slice(midpoint);
  const earlierAverage = earlier.length
    ? earlier.reduce((sum, row) => sum + row.value, 0) / earlier.length
    : null;
  const recentAverage = recent.length
    ? recent.reduce((sum, row) => sum + row.value, 0) / recent.length
    : null;

  const present = total(metrics["attendance.present"] ?? []);
  const attendanceTotal =
    present +
    total(metrics["attendance.absent"] ?? []) +
    total(metrics["attendance.late"] ?? []) +
    total(metrics["attendance.excused"] ?? []);

  return {
    averagePercentage: weightedAverage(scores),
    attendanceRate: attendanceTotal > 0 ? (present / attendanceTotal) * 100 : null,
    scoreRecords: total(metrics["assessments.count"] ?? []),
    newEnrolments: total(metrics["students.count"] ?? []),
    reportsPublished: total(metrics["reports.count"] ?? []),
    improvement:
      earlierAverage !== null && recentAverage !== null ? recentAverage - earlierAverage : null,
    dailyPerformance,
  };
}
