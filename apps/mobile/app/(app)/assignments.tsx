import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Assignment {
  id: string;
  title: string;
  instructions?: string | null;
  due_date?: string | null;
  max_score: number;
  status: string;
}

export default function StudentAssignments() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const enabled = features.has("assignments");
  const [items, setItems] = useState<Assignment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!client || !enabled) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const body = await client.get<{ data?: Assignment[] }>("/api/v1/assignments?limit=100");
      setItems(body.data ?? []);
    } catch {
      setError("Assignments could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  if (session?.user.role !== "student") return <Redirect href="/(app)" />;
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
          eyebrow="Learning workspace"
          title="Assignments"
          copy="Published work for your current class only."
        />
        {loading ? <LoadingState label="Loading assignments" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Assignments are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && items.length === 0 ? (
          <State
            title="Nothing due"
            copy="Published assignments from your teachers will appear here."
          />
        ) : null}
        {!loading && !error
          ? items.map((item) => (
              <View key={item.id} style={styles.card}>
                <View style={styles.row}>
                  <Text style={styles.name}>{item.title}</Text>
                  <Text style={styles.score}>{item.max_score} marks</Text>
                </View>
                {item.instructions ? (
                  <Text style={styles.instructions}>{item.instructions}</Text>
                ) : null}
                <Text style={styles.due}>
                  {item.due_date
                    ? `Due ${new Date(item.due_date).toLocaleDateString()}`
                    : "No due date"}
                </Text>
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
      <Text style={styles.instructions}>{copy}</Text>
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
  row: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  name: { flex: 1, color: colors.ink, fontSize: 17, fontWeight: "900" },
  score: { color: colors.ink, fontSize: 12, fontWeight: "900" },
  instructions: { color: colors.muted, lineHeight: 20 },
  due: { color: colors.ink, fontSize: 13, fontWeight: "800" },
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
