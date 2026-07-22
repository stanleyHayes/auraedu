import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Recommendation {
  id: string;
  recommendation_type: string;
  title: string;
  description?: string | null;
  status: "approved" | "overridden";
  confidence: number;
}

export default function StudentRecommendations() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const enabled = features.has("ai_recommendations");
  const [items, setItems] = useState<Recommendation[]>([]);
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
      // The service resolves the authenticated student's record through the
      // private Student Service scope API; no client-supplied learner ID is trusted.
      const body = await client.get<{ data?: Recommendation[] }>(
        "/api/v1/ai/recommendations?status=approved",
      );
      setItems(body.data ?? []);
    } catch {
      setError("Recommendations could not be refreshed.");
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
          eyebrow="Approved guidance"
          title="Recommendations"
          copy="Teacher-approved guidance generated from your learning signals."
        />
        {loading ? <LoadingState label="Loading recommendations" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="AI recommendations are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && items.length === 0 ? (
          <State
            title="No recommendations yet"
            copy="Approved learning guidance will appear here when it is ready."
          />
        ) : null}
        {!loading && !error
          ? items.map((item) => (
              <View key={item.id} style={styles.card}>
                <View style={styles.row}>
                  <Text style={styles.name}>{item.title}</Text>
                  <Text style={styles.confidence}>{Math.round(item.confidence * 100)}%</Text>
                </View>
                {item.description ? (
                  <Text style={styles.description}>{item.description}</Text>
                ) : null}
                <Text style={styles.kind}>{item.recommendation_type.replaceAll("_", " ")}</Text>
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
      <Text style={styles.description}>{copy}</Text>
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
  confidence: { color: colors.ink, fontSize: 12, fontWeight: "900" },
  description: { color: colors.muted, lineHeight: 20 },
  kind: { color: colors.ink, fontSize: 12, fontWeight: "800", textTransform: "capitalize" },
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
