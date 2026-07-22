import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Entry {
  id: string;
  subject_id: string;
  weekday: number;
  start_time: string;
  end_time: string;
  room?: string;
  status: string;
}
interface Subject {
  id: string;
  name: string;
  code?: string;
}
const days = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"];

export default function Timetable() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const [entries, setEntries] = useState<Entry[]>([]);
  const [subjects, setSubjects] = useState<Record<string, Subject>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const enabled = features.has("timetable");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client || !session) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [schedule, subjectList] = await Promise.all([
        client.get<{ data?: Entry[] }>("/api/v1/timetable?status=active&limit=100"),
        client.get<{ data?: Subject[] }>("/api/v1/subjects?limit=100"),
      ]);
      setEntries(schedule.data ?? []);
      setSubjects(
        Object.fromEntries((subjectList.data ?? []).map((subject) => [subject.id, subject])),
      );
    } catch {
      setError("Your timetable could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady, session]);
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  if (session && session.user.role !== "student") return <Redirect href="/(app)" />;
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
          eyebrow="Your school day"
          title="Timetable"
          copy="Your current class schedule, ordered by day and start time."
        />
        {loading ? <LoadingState label="Loading timetable" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Timetabling is not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && entries.length === 0 ? (
          <State
            title="No scheduled lessons"
            copy="Your school has not published a timetable for your class."
          />
        ) : null}
        {!loading && !error
          ? days.map((day, index) => {
              const periods = entries.filter((entry) => entry.weekday === index + 1);
              if (periods.length === 0) return null;
              return (
                <View key={day} style={styles.day}>
                  <Text style={styles.dayTitle}>{day}</Text>
                  {periods.map((entry) => (
                    <View key={entry.id} style={styles.period}>
                      <View style={styles.time}>
                        <Text style={styles.timeText}>{entry.start_time}</Text>
                        <Text style={styles.to}>to {entry.end_time}</Text>
                      </View>
                      <View style={styles.details}>
                        <Text style={styles.subject}>
                          {subjects[entry.subject_id]?.name ?? "Scheduled lesson"}
                        </Text>
                        <Text style={styles.meta}>
                          {entry.room ? `Room ${entry.room}` : "Room to be confirmed"}
                        </Text>
                      </View>
                    </View>
                  ))}
                </View>
              );
            })
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
  content: { gap: 15, paddingBottom: 36 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  intro: { color: colors.muted, lineHeight: 21, marginBottom: 5 },
  day: { gap: 9 },
  dayTitle: { color: colors.ink, fontSize: 17, fontWeight: "900", marginTop: 5 },
  period: {
    flexDirection: "row",
    gap: 15,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 16,
  },
  time: { width: 68, gap: 3 },
  timeText: { color: colors.ink, fontWeight: "900" },
  to: { color: colors.muted, fontSize: 12 },
  details: { flex: 1, gap: 5 },
  subject: { color: colors.ink, fontSize: 16, fontWeight: "900" },
  meta: { color: colors.muted, lineHeight: 20 },
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
