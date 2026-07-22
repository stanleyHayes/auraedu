import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, PrimaryButton, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

type Status = "present" | "absent" | "late" | "excused";
interface AttendanceRecord {
  id: string;
  student_id: string;
  date: string;
  status: Status;
  reason?: string | null;
}
interface Student {
  id: string;
  first_name: string;
  last_name: string;
}
interface SchoolClass {
  id: string;
  name: string;
  academic_year_id: string;
}
const statuses: Status[] = ["present", "absent", "late", "excused"];

export default function Attendance() {
  const { session } = useAuth();
  if (session?.user.role === "teacher") return <TeacherAttendance />;
  if (session?.user.role === "parent") return <ParentAttendance />;
  return <Redirect href="/(app)" />;
}

function ParentAttendance() {
  const { client, features, featuresReady } = useAuth();
  const theme = useTheme();
  const [records, setRecords] = useState<AttendanceRecord[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const enabled = features.has("attendance");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [attendance, family] = await Promise.all([
        client.get<{ data?: AttendanceRecord[] }>("/api/v1/attendance?limit=100"),
        client.get<{ students?: Student[] }>("/api/v1/guardians/me/children"),
      ]);
      setRecords(attendance.data ?? []);
      setNames(
        Object.fromEntries(
          (family.students ?? []).map((student) => [
            student.id,
            `${student.first_name} ${student.last_name}`,
          ]),
        ),
      );
    } catch {
      setError("Attendance could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  return (
    <Screen>
      <ScrollView
        contentContainerStyle={styles.content}
        refreshControl={
          <RefreshControl
            refreshing={loading}
            onRefresh={() => void load()}
            tintColor={theme.brand}
          />
        }
      >
        <Heading intro="Records for learners linked to your parent account only." />
        {loading ? <LoadingState label="Loading attendance" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Attendance is not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && records.length === 0 ? (
          <State
            title="No records yet"
            copy="Attendance will appear after your school records it."
          />
        ) : null}
        {!loading && !error
          ? records.map((record) => (
              <RecordCard
                key={record.id}
                record={record}
                name={names[record.student_id] ?? "Linked learner"}
              />
            ))
          : null}
      </ScrollView>
    </Screen>
  );
}

function TeacherAttendance() {
  const { client, features, featuresReady } = useAuth();
  const theme = useTheme();
  const enabled =
    features.has("attendance") &&
    features.has("academic_management") &&
    features.has("student_management");
  const today = useMemo(() => new Date().toISOString().slice(0, 10), []);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [selected, setSelected] = useState("");
  const [students, setStudents] = useState<Student[]>([]);
  const [marks, setMarks] = useState<Record<string, Status>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const loadClasses = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const body = await client.get<{ data?: SchoolClass[] }>("/api/v1/classes?limit=100");
      const next = body.data ?? [];
      setClasses(next);
      setSelected((current) =>
        current && next.some((item) => item.id === current) ? current : (next[0]?.id ?? ""),
      );
    } catch {
      setError("Assigned classes could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);

  const loadRoster = useCallback(async () => {
    if (!client || !selected) {
      setStudents([]);
      return;
    }
    setLoading(true);
    setError("");
    setNotice("");
    try {
      const [roster, attendance] = await Promise.all([
        client.get<{ data?: Student[] }>(
          `/api/v1/students?class_id=${encodeURIComponent(selected)}&limit=100`,
        ),
        client.get<{ data?: AttendanceRecord[] }>(`/api/v1/attendance?date=${today}&limit=100`),
      ]);
      const nextStudents = roster.data ?? [];
      const existing = new Map(
        (attendance.data ?? []).map((record) => [record.student_id, record.status]),
      );
      setStudents(nextStudents);
      setMarks(
        Object.fromEntries(
          nextStudents.map((student) => [student.id, existing.get(student.id) ?? "present"]),
        ),
      );
    } catch {
      setError("The assigned class register could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, selected, today]);

  useFocusEffect(
    useCallback(() => {
      void loadClasses();
    }, [loadClasses]),
  );
  useEffect(() => {
    void loadRoster();
  }, [loadRoster]);

  const submit = useCallback(async () => {
    if (!client || !selected || students.length === 0) return;
    const activeClass = classes.find((item) => item.id === selected);
    if (!activeClass) return;
    setSaving(true);
    setError("");
    setNotice("");
    try {
      await client.post("/api/v1/attendance/bulk", {
        date: today,
        academic_year_id: activeClass.academic_year_id,
        class_id: activeClass.id,
        records: students.map((student) => ({
          student_id: student.id,
          status: marks[student.id] ?? "present",
        })),
      });
      setNotice(`Attendance saved for ${students.length} learners.`);
    } catch {
      setError("Attendance was not saved. Refresh the register and try again.");
    } finally {
      setSaving(false);
    }
  }, [classes, client, marks, selected, students, today]);

  return (
    <Screen>
      <ScrollView
        contentContainerStyle={styles.content}
        refreshControl={
          <RefreshControl
            refreshing={loading}
            onRefresh={() => void loadRoster()}
            tintColor={theme.brand}
          />
        }
      >
        <Heading
          intro={`Your assigned registers for ${new Date(`${today}T00:00:00`).toLocaleDateString()}.`}
        />
        {featuresReady && !enabled ? (
          <State
            title="Not available"
            copy="Attendance, academics and student management must be enabled for teacher registers."
          />
        ) : null}
        {classes.length > 0 ? (
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.chips}
          >
            {classes.map((item) => (
              <Pressable
                key={item.id}
                accessibilityRole="radio"
                accessibilityState={{ checked: selected === item.id }}
                onPress={() => setSelected(item.id)}
                style={[
                  styles.chip,
                  selected === item.id && {
                    backgroundColor: theme.brand,
                    borderColor: theme.brand,
                  },
                ]}
              >
                <Text style={[styles.chipText, selected === item.id && { color: theme.onBrand }]}>
                  {item.name}
                </Text>
              </Pressable>
            ))}
          </ScrollView>
        ) : null}
        {loading ? <LoadingState label="Loading the class register" /> : null}
        {error ? <State title="Could not continue" copy={error} /> : null}
        {notice ? (
          <View accessibilityLiveRegion="polite" style={styles.success}>
            <Text style={styles.successText}>{notice}</Text>
          </View>
        ) : null}
        {!loading && enabled && classes.length === 0 && !error ? (
          <State
            title="No assigned classes"
            copy="Ask an administrator to link your staff profile and assign a class."
          />
        ) : null}
        {!loading && selected && students.length === 0 && !error ? (
          <State
            title="No active learners"
            copy="This assigned class has no active learners yet."
          />
        ) : null}
        {students.map((student) => (
          <View key={student.id} style={styles.markCard}>
            <View style={styles.studentCopy}>
              <Text style={styles.name}>
                {student.first_name} {student.last_name}
              </Text>
              <Text style={styles.date}>Tap a status to change it</Text>
            </View>
            <View style={styles.statuses}>
              {statuses.map((status) => (
                <Pressable
                  key={status}
                  accessibilityLabel={`${status} for ${student.first_name} ${student.last_name}`}
                  accessibilityRole="radio"
                  accessibilityState={{ checked: marks[student.id] === status }}
                  onPress={() => setMarks((current) => ({ ...current, [student.id]: status }))}
                  style={[styles.statusButton, marks[student.id] === status && statusStyle[status]]}
                >
                  <Text
                    style={[
                      styles.statusButtonText,
                      marks[student.id] === status && statusTextStyle[status],
                    ]}
                  >
                    {status.charAt(0).toUpperCase()}
                  </Text>
                </Pressable>
              ))}
            </View>
          </View>
        ))}
        {students.length > 0 ? (
          <PrimaryButton
            label={saving ? "Saving…" : `Save ${students.length} attendance marks`}
            disabled={saving || loading}
            onPress={() => void submit()}
          />
        ) : null}
      </ScrollView>
    </Screen>
  );
}

function Heading({ intro }: { intro: string }) {
  return <PageIntro eyebrow="Daily presence" title="Attendance" copy={intro} />;
}
function RecordCard({ record, name }: { record: AttendanceRecord; name: string }) {
  return (
    <View style={styles.card}>
      <View style={styles.row}>
        <Text style={styles.name}>{name}</Text>
        <Text style={[styles.status, statusStyle[record.status]]}>{record.status}</Text>
      </View>
      <Text style={styles.date}>{new Date(`${record.date}T00:00:00`).toLocaleDateString()}</Text>
      {record.reason ? <Text style={styles.reason}>{record.reason}</Text> : null}
    </View>
  );
}
function State({ title, copy }: { title: string; copy: string }) {
  return (
    <View accessibilityLiveRegion="polite" style={styles.state}>
      <Text style={styles.stateTitle}>{title}</Text>
      <Text style={styles.reason}>{copy}</Text>
    </View>
  );
}
const statusStyle = StyleSheet.create({
  present: { backgroundColor: "#E7F6EC" },
  absent: { backgroundColor: "#FDECEC" },
  late: { backgroundColor: "#FFF4D6" },
  excused: { backgroundColor: "#EAF0FF" },
});
const statusTextStyle = StyleSheet.create({
  present: { color: "#17663A" },
  absent: { color: colors.danger },
  late: { color: "#805B00" },
  excused: { color: "#294C9B" },
});
const styles = StyleSheet.create({
  content: { gap: 13, paddingBottom: 36 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  intro: { color: colors.muted, lineHeight: 21, marginBottom: 5 },
  card: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 17,
    gap: 7,
  },
  row: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 12 },
  name: { flex: 1, color: colors.ink, fontSize: 16, fontWeight: "900" },
  status: {
    overflow: "hidden",
    borderRadius: 999,
    paddingHorizontal: 10,
    paddingVertical: 5,
    fontSize: 12,
    fontWeight: "900",
    textTransform: "capitalize",
    color: colors.ink,
  },
  date: { color: colors.muted, fontSize: 13, fontWeight: "700" },
  reason: { color: colors.muted, lineHeight: 20 },
  state: {
    padding: 26,
    alignItems: "center",
    gap: 7,
    borderRadius: 16,
    borderColor: colors.border,
    borderWidth: 1,
    backgroundColor: colors.surface,
  },
  stateTitle: { color: colors.ink, fontSize: 18, fontWeight: "900" },
  chips: { gap: 9, paddingVertical: 2 },
  chip: {
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    paddingHorizontal: 15,
    paddingVertical: 10,
    borderRadius: 999,
  },
  chipText: { color: colors.ink, fontWeight: "800" },
  markCard: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 15,
    gap: 12,
  },
  studentCopy: { flexDirection: "row", alignItems: "center", gap: 10 },
  statuses: { flexDirection: "row", gap: 8 },
  statusButton: {
    flex: 1,
    minHeight: 44,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: 10,
    borderWidth: 1,
    borderColor: colors.border,
  },
  statusButtonText: { color: colors.muted, fontWeight: "900" },
  success: { borderRadius: 13, backgroundColor: "#E7F6EC", padding: 14 },
  successText: { color: "#17663A", fontWeight: "800" },
});
