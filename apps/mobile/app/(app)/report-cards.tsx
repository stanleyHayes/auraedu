import { Redirect, useFocusEffect } from "expo-router";
import * as FileSystem from "expo-file-system/legacy";
import * as Sharing from "expo-sharing";
import React, { useCallback, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { gatewayApiUrl, useAuth } from "../../src/auth";
import { LoadingState, PageIntro, PrimaryButton, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface ReportCard {
  id: string;
  student_id: string;
  academic_year_id?: string;
  term_id?: string;
  status: string;
  generated_at?: string;
}
interface Student {
  id: string;
  first_name: string;
  last_name: string;
}

export default function ReportCards() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const [cards, setCards] = useState<ReportCard[]>([]);
  const [students, setStudents] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [opening, setOpening] = useState("");
  const [error, setError] = useState("");
  const enabled = features.has("report_cards");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client || !session) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const response = await client.get<{ data?: ReportCard[] }>("/api/v1/report-cards?limit=100");
      setCards(response.data ?? []);
      if (session.user.role === "parent") {
        const family = await client.get<{ students?: Student[] }>("/api/v1/guardians/me/children");
        setStudents(
          Object.fromEntries(
            (family.students ?? []).map((student) => [
              student.id,
              `${student.first_name} ${student.last_name}`,
            ]),
          ),
        );
      } else if (session.user.role === "student") {
        const student = await client.get<Student>("/api/v1/students/me");
        setStudents({ [student.id]: `${student.first_name} ${student.last_name}` });
      } else {
        const assigned = await client.get<{ data?: Student[] }>("/api/v1/students?limit=100");
        setStudents(
          Object.fromEntries(
            (assigned.data ?? []).map((student) => [
              student.id,
              `${student.first_name} ${student.last_name}`,
            ]),
          ),
        );
      }
    } catch {
      setError("Published report cards could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady, session]);
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  const openPDF = useCallback(
    async (card: ReportCard) => {
      if (!session || !FileSystem.cacheDirectory) return;
      const localUri = `${FileSystem.cacheDirectory}auraedu-report-${card.id}.pdf`;
      setOpening(card.id);
      setError("");
      try {
        const result = await FileSystem.downloadAsync(
          `${gatewayApiUrl()}/api/v1/report-cards/${encodeURIComponent(card.id)}/download`,
          localUri,
          {
            headers: {
              Authorization: `Bearer ${session.accessToken}`,
              "X-Tenant-Code": session.user.tenant_id,
            },
          },
        );
        if (result.status < 200 || result.status >= 300) throw new Error("download failed");
        if (!(await Sharing.isAvailableAsync())) throw new Error("viewer unavailable");
        await Sharing.shareAsync(result.uri, {
          mimeType: "application/pdf",
          UTI: "com.adobe.pdf",
          dialogTitle: "Open report card",
        });
      } catch {
        setError("The secure PDF could not be opened on this device.");
      } finally {
        await FileSystem.deleteAsync(localUri, { idempotent: true }).catch(() => undefined);
        setOpening("");
      }
    },
    [session],
  );
  if (session && !["parent", "student", "teacher"].includes(session.user.role))
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
          eyebrow="Verified records"
          title="Report cards"
          copy={
            session?.user.role === "teacher"
              ? "Review report-card progress for learners in your assigned classes."
              : `Only published reports for ${session?.user.role === "parent" ? "your linked children" : "your student record"} are shown.`
          }
        />
        {loading ? <LoadingState label="Loading report cards" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Report cards are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && cards.length === 0 ? (
          <State
            title="No published reports"
            copy="Reports will appear after your school publishes them."
          />
        ) : null}
        {!loading
          ? cards.map((card) => (
              <View key={card.id} style={styles.card}>
                <View style={styles.row}>
                  <View style={styles.details}>
                    <Text style={styles.name}>{students[card.student_id] ?? "Linked learner"}</Text>
                    <Text style={styles.meta}>
                      {card.term_id ? `Term ${card.term_id.slice(0, 8)}` : "School report"} ·{" "}
                      {card.generated_at
                        ? new Date(card.generated_at).toLocaleDateString()
                        : "Published"}
                    </Text>
                  </View>
                  <Text
                    style={[
                      styles.badge,
                      { color: card.status === "published" ? theme.brand : colors.warning },
                    ]}
                  >
                    {card.status}
                  </Text>
                </View>
                {card.status === "published" ? (
                  <PrimaryButton
                    label={opening === card.id ? "Opening…" : "Open secure PDF"}
                    disabled={opening !== ""}
                    onPress={() => void openPDF(card)}
                  />
                ) : (
                  <Text style={styles.pending}>
                    PDF access appears after the school publishes this report.
                  </Text>
                )}
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
    gap: 12,
  },
  row: { flexDirection: "row", alignItems: "center", gap: 12 },
  details: { flex: 1, gap: 5 },
  name: { color: colors.ink, fontSize: 16, fontWeight: "900" },
  meta: { color: colors.muted, lineHeight: 20 },
  badge: { fontSize: 12, fontWeight: "900", textTransform: "uppercase" },
  pending: {
    color: colors.muted,
    borderTopColor: colors.border,
    borderTopWidth: 1,
    paddingTop: 12,
    fontSize: 13,
    lineHeight: 19,
  },
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
