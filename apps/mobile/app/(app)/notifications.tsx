import { useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import { Linking, RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, PrimaryButton, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Announcement {
  id: string;
  title: string;
  body: string;
  audience: "all" | "students" | "guardians" | "staff";
  created_at?: string;
}
const audienceByRole = { teacher: "staff", parent: "guardians", student: "students" } as const;

export default function Notifications() {
  const { client, enablePushNotifications, features, featuresReady, pushStatus, session } =
    useAuth();
  const theme = useTheme();
  const [items, setItems] = useState<Announcement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [pushBusy, setPushBusy] = useState(false);
  const [pushError, setPushError] = useState("");
  const enabled = features.has("announcements");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client || !session) {
      setItems([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const response = await client.get<{ data?: Announcement[] }>(
        "/api/v1/announcements?limit=50",
      );
      const audience = audienceByRole[session.user.role];
      setItems(
        (response.data ?? []).filter(
          (item) => item.audience === "all" || item.audience === audience,
        ),
      );
    } catch {
      setError("Notices could not be refreshed. Pull down to try again.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady, session]);
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
        <PageIntro
          eyebrow="In the loop"
          title="School notices"
          copy="Important updates for your role, without the noise."
        />
        {pushStatus !== "checking" && pushStatus !== "enabled" ? (
          <View accessibilityLiveRegion="polite" style={styles.permissionCard}>
            <View style={styles.permissionCopy}>
              <Text style={styles.permissionTitle}>
                {pushStatus === "blocked"
                  ? "Alerts are off on this device"
                  : "Get timely school alerts"}
              </Text>
              <Text style={styles.permissionText}>
                {pushStatus === "blocked"
                  ? "Open device settings to allow AuraEDU notifications. You can change this at any time."
                  : pushStatus === "unavailable"
                    ? "Push alerts are unavailable in this build. Notices remain available here."
                    : "Choose whether AuraEDU may send important school notices. We ask only when you enable them."}
              </Text>
              {pushError ? <Text style={styles.permissionError}>{pushError}</Text> : null}
            </View>
            {pushStatus === "available" ? (
              <PrimaryButton
                disabled={pushBusy}
                label={pushBusy ? "Enabling…" : "Enable push alerts"}
                onPress={() => {
                  setPushBusy(true);
                  setPushError("");
                  void enablePushNotifications()
                    .catch(() => setPushError("Push alerts could not be enabled. Try again."))
                    .finally(() => setPushBusy(false));
                }}
              />
            ) : null}
            {pushStatus === "blocked" ? (
              <PrimaryButton
                label="Open device settings"
                onPress={() => void Linking.openSettings()}
              />
            ) : null}
          </View>
        ) : null}
        {pushStatus === "enabled" ? (
          <View accessibilityLiveRegion="polite" style={styles.permissionEnabled}>
            <View style={[styles.enabledDot, { backgroundColor: theme.brand }]} />
            <Text style={styles.enabledCopy}>Push alerts are enabled for this device.</Text>
          </View>
        ) : null}
        {!featuresReady || loading ? (
          <View style={styles.state}>
            <LoadingState label="Checking for notices" />
          </View>
        ) : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Announcements are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && items.length === 0 ? (
          <State title="No new notices" copy="New announcements for your role will appear here." />
        ) : null}
        {!loading && !error
          ? items.map((item) => (
              <View key={item.id} style={styles.notice}>
                <View style={[styles.marker, { backgroundColor: theme.brand }]} />
                <View style={styles.noticeBody}>
                  <Text style={styles.noticeTitle}>{item.title}</Text>
                  <Text style={styles.copy}>{item.body}</Text>
                  {item.created_at ? (
                    <Text style={styles.date}>
                      {new Date(item.created_at).toLocaleDateString()}
                    </Text>
                  ) : null}
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
      <Text style={styles.copy}>{copy}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  content: { gap: 13, paddingBottom: 32 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  intro: { color: colors.muted, lineHeight: 21, marginBottom: 6 },
  state: {
    marginTop: 8,
    padding: 28,
    alignItems: "center",
    backgroundColor: colors.surface,
    borderColor: colors.border,
    borderWidth: 1,
    borderRadius: 16,
    gap: 8,
  },
  stateTitle: { color: colors.ink, fontSize: 18, fontWeight: "800" },
  copy: { color: colors.muted, lineHeight: 21 },
  notice: {
    flexDirection: "row",
    overflow: "hidden",
    backgroundColor: colors.surface,
    borderColor: colors.border,
    borderWidth: 1,
    borderRadius: 16,
  },
  marker: { width: 5 },
  noticeBody: { flex: 1, padding: 18, gap: 7 },
  noticeTitle: { color: colors.ink, fontSize: 17, fontWeight: "800" },
  date: { color: colors.muted, fontSize: 12, fontWeight: "700", marginTop: 3 },
  permissionCard: {
    backgroundColor: colors.midnight,
    borderRadius: 18,
    gap: 16,
    overflow: "hidden",
    padding: 20,
  },
  permissionCopy: { gap: 7 },
  permissionTitle: { color: "#FFFFFF", fontSize: 18, fontWeight: "900" },
  permissionText: { color: colors.ink200, lineHeight: 21 },
  permissionError: { color: colors.signal, fontSize: 13, fontWeight: "700" },
  permissionEnabled: {
    alignItems: "center",
    backgroundColor: colors.surface,
    borderColor: colors.border,
    borderRadius: 14,
    borderWidth: 1,
    flexDirection: "row",
    gap: 10,
    paddingHorizontal: 16,
    paddingVertical: 13,
  },
  enabledDot: { borderRadius: 99, height: 9, width: 9 },
  enabledCopy: { color: colors.ink, flex: 1, fontSize: 13, fontWeight: "700" },
});
