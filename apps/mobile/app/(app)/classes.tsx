import { Redirect, useFocusEffect, useRouter } from "expo-router";
import React, { useCallback, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, PrimaryButton, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface SchoolClass {
  id: string;
  name: string;
  status?: string;
}

export default function Classes() {
  const router = useRouter();
  const theme = useTheme();
  const { client, features, featuresReady, session } = useAuth();
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const enabled = features.has("academic_management");

  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const response = await client.get<{ data?: SchoolClass[] }>("/api/v1/classes?limit=100");
      setClasses(response.data ?? []);
    } catch {
      setError("Assigned classes could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );

  if (session && session.user.role !== "teacher") return <Redirect href="/(app)" />;

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
          eyebrow="Teaching workspace"
          title="My classes"
          copy="Only classes assigned to your staff identity are returned by the school workspace."
        />
        {loading ? <LoadingState label="Loading assigned classes" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Academic management is not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && classes.length === 0 ? (
          <State
            title="No assigned classes"
            copy="Ask a school administrator to link your staff account to a class."
          />
        ) : null}
        {!loading && !error
          ? classes.map((item, index) => (
              <View key={item.id} style={styles.card}>
                <View
                  style={[
                    styles.number,
                    { backgroundColor: index === 0 ? theme.brand : colors.midnight },
                  ]}
                >
                  <Text
                    style={[
                      styles.numberText,
                      { color: index === 0 ? theme.onBrand : colors.paper },
                    ]}
                  >
                    {String(index + 1).padStart(2, "0")}
                  </Text>
                </View>
                <View style={styles.copy}>
                  <Text style={styles.name}>{item.name}</Text>
                  <Text style={styles.meta}>
                    {item.status === "inactive" ? "Inactive assignment" : "Active teaching group"}
                  </Text>
                </View>
                <View style={styles.action}>
                  <PrimaryButton
                    label="Open register"
                    onPress={() => router.push("/(app)/attendance")}
                  />
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
  content: { gap: 13, paddingBottom: 36 },
  card: {
    flexDirection: "row",
    flexWrap: "wrap",
    alignItems: "center",
    gap: 13,
    borderRadius: 18,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 17,
    shadowColor: colors.midnight,
    shadowOpacity: 0.06,
    shadowRadius: 14,
    shadowOffset: { width: 0, height: 7 },
    elevation: 2,
  },
  number: {
    width: 44,
    height: 44,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: 14,
  },
  numberText: { fontSize: 12, fontWeight: "900", letterSpacing: 0.8 },
  copy: { flex: 1, minWidth: 150, gap: 4 },
  name: { color: colors.ink, fontSize: 17, fontWeight: "900" },
  meta: { color: colors.muted, fontSize: 13, lineHeight: 19 },
  action: { width: "100%", paddingTop: 2 },
  state: {
    padding: 26,
    alignItems: "center",
    gap: 7,
    borderRadius: 18,
    borderColor: colors.border,
    borderWidth: 1,
    backgroundColor: colors.surface,
  },
  stateTitle: { color: colors.ink, fontSize: 18, fontWeight: "900" },
});
