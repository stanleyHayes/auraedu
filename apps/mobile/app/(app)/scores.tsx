import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useEffect, useState } from "react";
import {
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, PrimaryButton, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Assessment {
  id: string;
  title: string;
  type: string;
  max_score: number;
  class_ids?: string[];
  status: string;
}
interface Student {
  id: string;
  first_name: string;
  last_name: string;
}
interface Score {
  id: string;
  student_id: string;
  score: number;
}

export default function TeacherScores() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const enabled = features.has("assessments") && features.has("student_management");
  const [assessments, setAssessments] = useState<Assessment[]>([]);
  const [selected, setSelected] = useState("");
  const [students, setStudents] = useState<Student[]>([]);
  const [existing, setExisting] = useState<Record<string, Score>>({});
  const [values, setValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const loadAssessments = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const body = await client.get<{ data?: Assessment[] }>("/api/v1/assessments?limit=100");
      const next = body.data ?? [];
      setAssessments(next);
      setSelected((current) =>
        current && next.some((item) => item.id === current) ? current : (next[0]?.id ?? ""),
      );
    } catch {
      setError("Assigned assessments could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);

  const loadGradebook = useCallback(async () => {
    if (!client || !selected) {
      setStudents([]);
      return;
    }
    const assessment = assessments.find((item) => item.id === selected);
    if (!assessment) return;
    setLoading(true);
    setError("");
    setNotice("");
    try {
      const [rosters, scoreBody] = await Promise.all([
        Promise.all(
          (assessment.class_ids ?? []).map((classID) =>
            client.get<{ data?: Student[] }>(
              `/api/v1/students?class_id=${encodeURIComponent(classID)}&limit=100`,
            ),
          ),
        ),
        client.get<{ data?: Score[] }>(`/api/v1/assessments/${assessment.id}/scores?limit=100`),
      ]);
      const unique = new Map<string, Student>();
      for (const roster of rosters)
        for (const student of roster.data ?? []) unique.set(student.id, student);
      const nextStudents = Array.from(unique.values()).sort((a, b) =>
        `${a.last_name}${a.first_name}`.localeCompare(`${b.last_name}${b.first_name}`),
      );
      const scoreMap = Object.fromEntries(
        (scoreBody.data ?? []).map((score) => [score.student_id, score]),
      );
      setStudents(nextStudents);
      setExisting(scoreMap);
      setValues(
        Object.fromEntries(
          nextStudents.map((student) => {
            const saved = scoreMap[student.id];
            return [student.id, saved ? String(saved.score) : ""];
          }),
        ),
      );
    } catch {
      setError("The assigned gradebook could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [assessments, client, selected]);

  useFocusEffect(
    useCallback(() => {
      void loadAssessments();
    }, [loadAssessments]),
  );
  useEffect(() => {
    void loadGradebook();
  }, [loadGradebook]);

  const submit = useCallback(async () => {
    if (!client || !session) return;
    const assessment = assessments.find((item) => item.id === selected);
    if (!assessment) return;
    const entries = students.flatMap((student) => {
      const raw = values[student.id]?.trim() ?? "";
      if (raw === "") return [];
      const score = Number(raw);
      return Number.isInteger(score) && score >= 0 && score <= assessment.max_score
        ? [{ student, score }]
        : [];
    });
    const invalid = students.some((student) => {
      const raw = values[student.id]?.trim() ?? "";
      if (raw === "") return false;
      const score = Number(raw);
      return !Number.isInteger(score) || score < 0 || score > assessment.max_score;
    });
    if (invalid) {
      setError(`Scores must be whole numbers from 0 to ${assessment.max_score}.`);
      return;
    }
    setSaving(true);
    setError("");
    setNotice("");
    try {
      await Promise.all(
        entries.map(({ student, score }) => {
          const saved = existing[student.id];
          return saved
            ? client.patch(`/api/v1/assessments/${assessment.id}/scores/${saved.id}`, { score })
            : client.post(`/api/v1/assessments/${assessment.id}/scores`, {
                student_id: student.id,
                score,
                recorded_by: session.user.id,
                notes: "",
              });
        }),
      );
      await loadGradebook();
      setNotice(`Saved ${entries.length} scores for ${assessment.title}.`);
    } catch {
      setError("Scores were not saved. Refresh the gradebook and try again.");
    } finally {
      setSaving(false);
    }
  }, [assessments, client, existing, loadGradebook, selected, session, students, values]);

  if (session?.user.role !== "teacher") return <Redirect href="/(app)" />;
  const active = assessments.find((item) => item.id === selected);
  return (
    <Screen>
      <ScrollView
        contentContainerStyle={styles.content}
        refreshControl={
          <RefreshControl
            refreshing={loading}
            onRefresh={() => void loadGradebook()}
            tintColor={theme.brand}
          />
        }
      >
        <PageIntro
          eyebrow="Teaching workspace"
          title="Record scores"
          copy="Only assessments and learners in your assigned classes are returned by the server."
        />
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Assessments and student management must be enabled." />
        ) : null}
        {assessments.length > 0 ? (
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.chips}
          >
            {assessments.map((item) => (
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
                  {item.title}
                </Text>
              </Pressable>
            ))}
          </ScrollView>
        ) : null}
        {active ? (
          <Text style={styles.context}>
            {active.type} · maximum {active.max_score}
          </Text>
        ) : null}
        {loading ? <LoadingState label="Loading the gradebook" /> : null}
        {error ? <State title="Could not continue" copy={error} /> : null}
        {notice ? (
          <View accessibilityLiveRegion="polite" style={styles.success}>
            <Text style={styles.successText}>{notice}</Text>
          </View>
        ) : null}
        {!loading && enabled && assessments.length === 0 && !error ? (
          <State
            title="No assigned assessments"
            copy="Published or draft assessments for your assigned classes will appear here."
          />
        ) : null}
        {!loading && active && students.length === 0 && !error ? (
          <State
            title="No active learners"
            copy="This assessment has no active learners in your assigned classes."
          />
        ) : null}
        {students.map((student) => (
          <View key={student.id} style={styles.row}>
            <View style={styles.student}>
              <Text style={styles.name}>
                {student.first_name} {student.last_name}
              </Text>
              <Text style={styles.saved}>{existing[student.id] ? "Recorded" : "Not recorded"}</Text>
            </View>
            <TextInput
              value={values[student.id] ?? ""}
              onChangeText={(value) =>
                setValues((current) => ({ ...current, [student.id]: value.replace(/[^0-9]/g, "") }))
              }
              keyboardType="number-pad"
              placeholder="—"
              placeholderTextColor={colors.muted}
              style={styles.input}
              accessibilityLabel={`Score for ${student.first_name} ${student.last_name}`}
            />
            <Text style={styles.max}>/ {active?.max_score ?? 0}</Text>
          </View>
        ))}
        {students.length > 0 ? (
          <PrimaryButton
            label={saving ? "Saving…" : "Save entered scores"}
            disabled={saving || loading}
            onPress={() => void submit()}
          />
        ) : null}
      </ScrollView>
    </Screen>
  );
}

function State({ title, copy }: { title: string; copy: string }) {
  return (
    <View accessibilityLiveRegion="polite" style={styles.state}>
      <Text style={styles.stateTitle}>{title}</Text>
      <Text style={styles.intro}>{copy}</Text>
    </View>
  );
}
const styles = StyleSheet.create({
  content: { gap: 13, paddingBottom: 36 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  intro: { color: colors.muted, lineHeight: 21 },
  chips: { gap: 9, paddingVertical: 2 },
  chip: {
    overflow: "hidden",
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    paddingHorizontal: 15,
    paddingVertical: 10,
    borderRadius: 999,
  },
  chipText: { color: colors.ink, fontWeight: "800" },
  context: { color: colors.ink, fontWeight: "800", textTransform: "capitalize" },
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 14,
  },
  student: { flex: 1, gap: 4 },
  name: { color: colors.ink, fontSize: 15, fontWeight: "900" },
  saved: { color: colors.muted, fontSize: 12 },
  input: {
    width: 58,
    minHeight: 44,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.paper,
    color: colors.ink,
    textAlign: "center",
    fontSize: 17,
    fontWeight: "900",
  },
  max: { color: colors.muted, fontWeight: "700" },
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
  success: { borderRadius: 13, backgroundColor: "#E7F6EC", padding: 14 },
  successText: { color: "#17663A", fontWeight: "800" },
});
