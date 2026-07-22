import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Student {
  id: string;
  first_name: string;
  last_name: string;
  admission_number?: string;
  status?: string;
}
interface GuardianChildren {
  students: Student[];
}

export default function Children() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const enabled = features.has("student_management");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const response = await client.get<GuardianChildren>("/api/v1/guardians/me/children");
      setStudents(response.students ?? []);
    } catch {
      setError("Linked learners could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  if (session?.user.role !== "parent") return <Redirect href="/(app)" />;
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
          eyebrow="Family workspace"
          title="My children"
          copy="Learners securely linked to your parent account."
        />
        {loading ? <LoadingState label="Loading linked learners" /> : null}
        {!enabled && featuresReady ? (
          <State title="Not available" copy="Student profiles are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && students.length === 0 ? (
          <State
            title="No learners linked"
            copy="Ask your school to link your parent account to your children."
          />
        ) : null}
        {!loading && !error
          ? students.map((student) => (
              <View key={student.id} style={styles.card}>
                <View style={[styles.avatar, { backgroundColor: theme.brand }]}>
                  <Text style={[styles.initials, { color: theme.onBrand }]}>
                    {student.first_name.slice(0, 1)}
                    {student.last_name.slice(0, 1)}
                  </Text>
                </View>
                <View style={styles.details}>
                  <Text style={styles.name}>
                    {student.first_name} {student.last_name}
                  </Text>
                  <Text style={styles.meta}>
                    {student.admission_number ?? "Admission number pending"}
                  </Text>
                  {student.status ? <Text style={styles.status}>{student.status}</Text> : null}
                </View>
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
  content: { gap: 14, paddingBottom: 36 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  intro: { color: colors.muted, lineHeight: 21, marginBottom: 4 },
  card: {
    flexDirection: "row",
    alignItems: "center",
    gap: 14,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 17,
  },
  avatar: {
    width: 48,
    height: 48,
    borderRadius: 24,
    alignItems: "center",
    justifyContent: "center",
  },
  initials: { fontSize: 15, fontWeight: "900" },
  details: { flex: 1, gap: 4 },
  name: { color: colors.ink, fontSize: 17, fontWeight: "900" },
  meta: { color: colors.muted, lineHeight: 20 },
  status: { color: colors.ink, fontSize: 12, fontWeight: "800", textTransform: "capitalize" },
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
