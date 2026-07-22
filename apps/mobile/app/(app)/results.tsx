import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Assessment {
  id: string;
  name: string;
  type: string;
  max_score?: number;
  status?: string;
}
interface Score {
  id: string;
  student_id: string;
  score: number;
  notes?: string | null;
}
interface Result {
  key: string;
  assessment: Assessment;
  score: Score;
}
interface Student {
  id: string;
  first_name: string;
  last_name: string;
}

export default function Results() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const [results, setResults] = useState<Result[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const enabled = features.has("assessments");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client || !session) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const assessments = await client.get<{ data?: Assessment[] }>(
        "/api/v1/assessments?status=published&limit=50",
      );
      const scoreGroups = await Promise.all(
        (assessments.data ?? []).map(async (assessment) => {
          const response = await client.get<{ data?: Score[] }>(
            `/api/v1/assessments/${assessment.id}/scores?limit=100`,
          );
          return (response.data ?? []).map((score) => ({
            key: `${assessment.id}-${score.id}`,
            assessment,
            score,
          }));
        }),
      );
      setResults(scoreGroups.flat());
      if (session.user.role === "parent") {
        const family = await client.get<{ students?: Student[] }>("/api/v1/guardians/me/children");
        setNames(
          Object.fromEntries(
            (family.students ?? []).map((student) => [
              student.id,
              `${student.first_name} ${student.last_name}`,
            ]),
          ),
        );
      } else {
        const student = await client.get<Student>("/api/v1/students/me");
        setNames({ [student.id]: `${student.first_name} ${student.last_name}` });
      }
    } catch {
      setError("Published results could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady, session]);
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  if (session && session.user.role !== "parent" && session.user.role !== "student")
    return <Redirect href="/(app)" />;
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
        <PageIntro
          eyebrow="Published learning"
          title="Results"
          copy={`Published scores for ${session?.user.role === "parent" ? "your linked children" : "your student record"}.`}
        />
        {loading ? <LoadingState label="Loading results" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Assessments are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && results.length === 0 ? (
          <State
            title="No published results"
            copy="Results will appear after your school publishes them."
          />
        ) : null}
        {!loading && !error
          ? results.map((result) => (
              <View key={result.key} style={styles.card}>
                <View style={styles.row}>
                  <View style={styles.details}>
                    <Text style={styles.name}>{result.assessment.name}</Text>
                    <Text style={styles.meta}>
                      {names[result.score.student_id] ?? "Linked learner"} ·{" "}
                      {result.assessment.type}
                    </Text>
                  </View>
                  <Text style={styles.score}>
                    {result.score.score}
                    <Text style={styles.maximum}> / {result.assessment.max_score ?? "—"}</Text>
                  </Text>
                </View>
                {result.score.notes ? <Text style={styles.meta}>{result.score.notes}</Text> : null}
              </View>
            ))
          : null}
      </ScrollView>
    </Screen>
  );
}
function State({ title, copy }: { title: string; copy: string }) {
  return (
    <View accessibilityLiveRegion="polite" style={styles.state}>
      <Text style={styles.stateTitle}>{title}</Text>
      <Text style={styles.meta}>{copy}</Text>
    </View>
  );
}
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
    gap: 9,
  },
  row: { flexDirection: "row", alignItems: "center", gap: 12 },
  details: { flex: 1, gap: 5 },
  name: { color: colors.ink, fontSize: 16, fontWeight: "900" },
  meta: { color: colors.muted, lineHeight: 20, textTransform: "capitalize" },
  score: { color: colors.ink, fontSize: 20, fontWeight: "900" },
  maximum: { color: colors.muted, fontSize: 13, fontWeight: "700" },
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
});
