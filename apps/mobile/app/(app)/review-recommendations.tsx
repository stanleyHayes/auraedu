import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useEffect, useState } from "react";
import { Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, PrimaryButton, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface ClassRecord {
  id: string;
  name: string;
}
interface Student {
  id: string;
  first_name: string;
  last_name: string;
}
interface Recommendation {
  id: string;
  recommendation_type: string;
  title: string;
  description?: string | null;
  status: "pending" | "approved" | "rejected" | "overridden";
  confidence: number;
}

export default function ReviewRecommendations() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const enabled =
    features.has("ai_recommendations") &&
    features.has("student_management") &&
    features.has("academic_management");
  const [classes, setClasses] = useState<ClassRecord[]>([]);
  const [classID, setClassID] = useState("");
  const [students, setStudents] = useState<Student[]>([]);
  const [studentID, setStudentID] = useState("");
  const [items, setItems] = useState<Recommendation[]>([]);
  const [loading, setLoading] = useState(true);
  const [reviewing, setReviewing] = useState("");
  const [error, setError] = useState("");

  const loadClasses = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const body = await client.get<{ data?: ClassRecord[] }>("/api/v1/classes?limit=100");
      const next = body.data ?? [];
      setClasses(next);
      setClassID((current) =>
        current && next.some((item) => item.id === current) ? current : (next[0]?.id ?? ""),
      );
    } catch {
      setError("Assigned classes could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);

  const loadStudents = useCallback(async () => {
    if (!client || !classID) {
      setStudents([]);
      setStudentID("");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const body = await client.get<{ data?: Student[] }>(
        `/api/v1/students?class_id=${encodeURIComponent(classID)}&limit=100`,
      );
      const next = (body.data ?? []).sort((a, b) =>
        `${a.last_name}${a.first_name}`.localeCompare(`${b.last_name}${b.first_name}`),
      );
      setStudents(next);
      setStudentID((current) =>
        current && next.some((item) => item.id === current) ? current : (next[0]?.id ?? ""),
      );
    } catch {
      setError("The assigned class roster could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [classID, client]);

  const loadRecommendations = useCallback(async () => {
    if (!client || !studentID) {
      setItems([]);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const body = await client.get<{ data?: Recommendation[] }>(
        `/api/v1/ai/recommendations?student_id=${encodeURIComponent(studentID)}`,
      );
      setItems(body.data ?? []);
    } catch {
      setError("Recommendations for this learner could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [client, studentID]);

  useFocusEffect(
    useCallback(() => {
      void loadClasses();
    }, [loadClasses]),
  );
  useEffect(() => {
    void loadStudents();
  }, [loadStudents]);
  useEffect(() => {
    void loadRecommendations();
  }, [loadRecommendations]);

  const review = useCallback(
    async (id: string, action: "approve" | "reject") => {
      if (!client) return;
      setReviewing(id);
      setError("");
      try {
        await client.post(`/api/v1/ai/recommendations/${id}/${action}`, {});
        await loadRecommendations();
      } catch {
        setError(
          `The recommendation could not be ${action === "approve" ? "approved" : "rejected"}.`,
        );
      } finally {
        setReviewing("");
      }
    },
    [client, loadRecommendations],
  );

  if (session?.user.role !== "teacher") return <Redirect href="/(app)" />;
  return (
    <Screen>
      <ScrollView
        contentContainerStyle={styles.content}
        refreshControl={
          <RefreshControl
            refreshing={loading}
            onRefresh={() => void loadRecommendations()}
            tintColor={theme.brand}
          />
        }
      >
        <PageIntro
          eyebrow="Teacher review"
          title="Review guidance"
          copy="Only learners in your assigned classes are available. AI suggestions stay hidden from families until a teacher approves them."
        />
        {featuresReady && !enabled ? (
          <State
            title="Not available"
            copy="AI recommendations, academics and student management must be enabled."
          />
        ) : null}
        {classes.length > 0 ? (
          <>
            <Text style={styles.label}>Class</Text>
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              contentContainerStyle={styles.chips}
            >
              {classes.map((item) => (
                <Pressable
                  key={item.id}
                  accessibilityRole="radio"
                  accessibilityState={{ checked: classID === item.id }}
                  onPress={() => setClassID(item.id)}
                  style={[
                    styles.chip,
                    classID === item.id && {
                      backgroundColor: theme.brand,
                      borderColor: theme.brand,
                    },
                  ]}
                >
                  <Text style={[styles.chipText, classID === item.id && { color: theme.onBrand }]}>
                    {item.name}
                  </Text>
                </Pressable>
              ))}
            </ScrollView>
          </>
        ) : null}
        {students.length > 0 ? (
          <>
            <Text style={styles.label}>Learner</Text>
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              contentContainerStyle={styles.chips}
            >
              {students.map((student) => (
                <Pressable
                  key={student.id}
                  accessibilityRole="radio"
                  accessibilityState={{ checked: studentID === student.id }}
                  onPress={() => setStudentID(student.id)}
                  style={[
                    styles.chip,
                    studentID === student.id && {
                      backgroundColor: theme.brand,
                      borderColor: theme.brand,
                    },
                  ]}
                >
                  <Text
                    style={[styles.chipText, studentID === student.id && { color: theme.onBrand }]}
                  >
                    {student.first_name} {student.last_name}
                  </Text>
                </Pressable>
              ))}
            </ScrollView>
          </>
        ) : null}
        {loading ? <LoadingState label="Loading review guidance" /> : null}
        {error ? <State title="Could not continue" copy={error} /> : null}
        {!loading && enabled && classes.length === 0 && !error ? (
          <State
            title="No assigned classes"
            copy="Classes assigned to your teacher account will appear here."
          />
        ) : null}
        {!loading && classID && students.length === 0 && !error ? (
          <State title="No active learners" copy="This assigned class has no active learners." />
        ) : null}
        {!loading && studentID && items.length === 0 && !error ? (
          <State
            title="No guidance yet"
            copy="Generated learning suggestions for this learner will appear here for review."
          />
        ) : null}
        {items.map((item) => (
          <View key={item.id} style={styles.card}>
            <View style={styles.row}>
              <Text style={styles.name}>{item.title}</Text>
              <Text style={[styles.status, item.status === "pending" && { color: theme.brand }]}>
                {item.status}
              </Text>
            </View>
            {item.description ? <Text style={styles.description}>{item.description}</Text> : null}
            <Text style={styles.meta}>
              {item.recommendation_type.replaceAll("_", " ")} · {Math.round(item.confidence * 100)}%
              confidence
            </Text>
            {item.status === "pending" ? (
              <View style={styles.actions}>
                <View style={styles.action}>
                  <PrimaryButton
                    label={reviewing === item.id ? "Saving…" : "Approve"}
                    disabled={reviewing !== ""}
                    onPress={() => void review(item.id, "approve")}
                  />
                </View>
                <Pressable
                  accessibilityRole="button"
                  accessibilityState={{ disabled: reviewing !== "" }}
                  disabled={reviewing !== ""}
                  onPress={() => reviewing === "" && void review(item.id, "reject")}
                  style={styles.reject}
                >
                  <Text style={styles.rejectText}>Reject</Text>
                </Pressable>
              </View>
            ) : null}
          </View>
        ))}
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
  label: {
    color: colors.ink,
    fontSize: 12,
    fontWeight: "900",
    letterSpacing: 1,
    textTransform: "uppercase",
    marginTop: 4,
  },
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
  card: {
    gap: 10,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 16,
  },
  row: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  name: { flex: 1, color: colors.ink, fontSize: 17, fontWeight: "900" },
  status: { color: colors.muted, fontSize: 12, fontWeight: "900", textTransform: "uppercase" },
  description: { color: colors.ink, lineHeight: 21 },
  meta: { color: colors.muted, fontSize: 12, textTransform: "capitalize" },
  actions: { flexDirection: "row", alignItems: "center", gap: 18, marginTop: 3 },
  action: { flex: 1 },
  reject: { minHeight: 44, justifyContent: "center", paddingHorizontal: 12 },
  rejectText: { color: "#9A3412", fontWeight: "900" },
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
