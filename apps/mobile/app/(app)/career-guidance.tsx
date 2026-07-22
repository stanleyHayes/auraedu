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
}
interface Guidance {
  id: string;
  student_id: string;
  guidance_type: string;
  title: string;
  value: number;
  confidence: number;
  status: "approved";
  explanation?: string | null;
}

export default function CareerGuidance() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const enabled = features.has("career_guidance");
  const [items, setItems] = useState<Guidance[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client || !session) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      if (session.user.role === "student") {
        const body = await client.get<{ data?: Guidance[] }>("/api/v1/ai/career-guidance/guidance");
        setItems(body.data ?? []);
        setNames({});
      } else {
        const childBody = await client.get<{ data?: Student[] }>("/api/v1/guardians/me/students");
        const children = childBody.data ?? [];
        const responses = await Promise.all(
          children.map((child) =>
            client.get<{ data?: Guidance[] }>(
              `/api/v1/ai/career-guidance/guidance?student_id=${encodeURIComponent(child.id)}`,
            ),
          ),
        );
        setItems(responses.flatMap((body) => body.data ?? []));
        setNames(
          Object.fromEntries(
            children.map((child) => [child.id, `${child.first_name} ${child.last_name}`]),
          ),
        );
      }
    } catch {
      setError("Approved career guidance could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady, session]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  if (session && session.user.role !== "student" && session.user.role !== "parent")
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
          eyebrow="Future pathways"
          title="Career guidance"
          copy="Teacher-approved pathways built from learning signals. Guidance supports decisions; it does not make them for the learner."
        />
        {loading ? <LoadingState label="Loading career guidance" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Career guidance is not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && items.length === 0 ? (
          <State
            title="No approved guidance"
            copy="Reviewed pathways and study suggestions will appear here when they are ready."
          />
        ) : null}
        {!loading && !error
          ? items.map((item) => (
              <View key={item.id} style={styles.card}>
                <View style={styles.row}>
                  <Text style={styles.name}>{item.title}</Text>
                  <Text style={[styles.confidence, { color: theme.brand }]}>
                    {Math.round(item.confidence * 100)}%
                  </Text>
                </View>
                {names[item.student_id] ? (
                  <Text style={styles.learner}>{names[item.student_id]}</Text>
                ) : null}
                {item.explanation ? (
                  <Text style={styles.description}>{item.explanation}</Text>
                ) : null}
                <Text style={styles.kind}>{item.guidance_type.replaceAll("_", " ")}</Text>
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
      <Text style={styles.intro}>{copy}</Text>
    </View>
  );
}
const styles = StyleSheet.create({
  content: { gap: 13, paddingBottom: 36 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  intro: { color: colors.muted, lineHeight: 21 },
  card: {
    gap: 9,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 16,
  },
  row: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  name: { flex: 1, color: colors.ink, fontSize: 17, fontWeight: "900" },
  confidence: { fontSize: 13, fontWeight: "900" },
  learner: { color: colors.ink, fontSize: 13, fontWeight: "800" },
  description: { color: colors.ink, lineHeight: 21 },
  kind: { color: colors.muted, fontSize: 12, fontWeight: "800", textTransform: "capitalize" },
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
