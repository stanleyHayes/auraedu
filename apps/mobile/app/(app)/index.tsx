import { Image, ScrollView, StyleSheet, Text, View } from "react-native";
import { Eyebrow, ModuleCard, Screen } from "../../src/components";
import { useAuth } from "../../src/auth";
import { colors, useTheme } from "../../src/theme";

const roleCopy = {
  teacher: [
    "Teaching workspace",
    "Classes, attendance, scores and reports—without the admin console.",
  ],
  parent: [
    "Family workspace",
    "Children, attendance, results, fees and school notices in one view.",
  ],
  student: [
    "Learning workspace",
    "Timetable, assignments, results and approved guidance for your next step.",
  ],
} as const;

export default function Today() {
  const { session, tenantCode, branding } = useAuth();
  const theme = useTheme();
  const role = session?.user.role ?? "student";
  const copy = roleCopy[role];
  return (
    <Screen>
      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.school}>
          {branding?.logoUrl ? (
            <Image
              accessibilityLabel={`${branding.name} logo`}
              source={{ uri: branding.logoUrl }}
              style={styles.logo}
              resizeMode="contain"
            />
          ) : null}
          <View style={styles.schoolCopy}>
            <Eyebrow>
              {branding?.short ?? tenantCode} · {role}
            </Eyebrow>
            <Text style={styles.schoolName}>{branding?.name ?? tenantCode}</Text>
          </View>
        </View>
        <View style={styles.heroCopy}>
          <Text style={styles.kicker}>YOUR OPERATING BRIEF</Text>
          <Text accessibilityRole="header" style={styles.title}>
            Good day, {session?.user.name.split(" ")[0] ?? "there"}.
          </Text>
          <Text style={styles.subtitle}>{copy[1]}</Text>
        </View>
        <View style={styles.callout}>
          <View
            pointerEvents="none"
            style={[styles.calloutOrb, { backgroundColor: theme.brand }]}
          />
          <Text style={[styles.calloutLabel, { color: theme.brand }]}>TODAY</Text>
          <Text style={styles.calloutTitle}>{copy[0]}</Text>
          <Text style={styles.calloutCopy}>
            Your school controls which modules appear. Disabled features stay unavailable even
            through direct links.
          </Text>
        </View>
        <View style={styles.sectionRow}>
          <Text style={styles.section}>Ready for you</Text>
          <Text style={styles.sectionHint}>ROLE-AWARE</Text>
        </View>
        {role === "teacher" ? (
          <>
            <ModuleCard
              title="Take attendance"
              copy="Open an assigned class register and submit one clear daily record."
              href="/(app)/attendance"
            />
            <ModuleCard
              title="Record scores"
              copy="Enter assessment results for your assigned classes."
              href="/(app)/scores"
            />
          </>
        ) : role === "parent" ? (
          <>
            <ModuleCard
              title="Children"
              copy="See linked learners and their latest school activity."
              href="/(app)/children"
            />
            <ModuleCard
              title="Fees"
              copy="Review balances and start a secure school payment."
              href="/(app)/fees"
            />
            <ModuleCard
              title="Report cards"
              copy="Open published learner reports securely."
              href="/(app)/report-cards"
            />
          </>
        ) : (
          <>
            <ModuleCard
              title="Assignments"
              copy="See work published by your teachers."
              href="/(app)/assignments"
            />
            <ModuleCard
              title="Results"
              copy="Review released scores and report cards."
              href="/(app)/results"
            />
          </>
        )}
      </ScrollView>
    </Screen>
  );
}

const styles = StyleSheet.create({
  content: { paddingBottom: 116, gap: 14 },
  school: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    padding: 10,
    marginHorizontal: -4,
    borderRadius: 16,
    backgroundColor: "rgba(255,255,255,0.58)",
  },
  logo: { width: 42, height: 42 },
  schoolCopy: { flex: 1, gap: 3 },
  schoolName: { color: colors.ink, fontSize: 14, fontWeight: "800" },
  heroCopy: { gap: 8, paddingTop: 8 },
  kicker: { color: colors.teal, fontSize: 10, fontWeight: "900", letterSpacing: 1.6 },
  title: { color: colors.ink, fontSize: 34, lineHeight: 39, fontWeight: "900", letterSpacing: -1 },
  subtitle: { color: colors.muted, fontSize: 16, lineHeight: 23, marginBottom: 8 },
  callout: {
    backgroundColor: colors.midnight,
    borderRadius: 24,
    padding: 23,
    gap: 8,
    overflow: "hidden",
    shadowColor: colors.midnight,
    shadowOpacity: 0.2,
    shadowRadius: 20,
    shadowOffset: { width: 0, height: 10 },
    elevation: 5,
  },
  calloutOrb: {
    position: "absolute",
    width: 150,
    height: 150,
    borderRadius: 90,
    right: -62,
    top: -70,
    opacity: 0.32,
  },
  calloutLabel: { fontSize: 11, letterSpacing: 1.5, fontWeight: "900" },
  calloutTitle: { color: colors.paper, fontSize: 23, fontWeight: "900" },
  calloutCopy: { color: "#C9D2CC", lineHeight: 21 },
  sectionRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginTop: 12,
  },
  section: { color: colors.ink, fontSize: 18, fontWeight: "900" },
  sectionHint: { color: colors.muted, fontSize: 9, fontWeight: "900", letterSpacing: 1.3 },
});
