import { ScrollView, StyleSheet } from "react-native";
import { ModuleCard, PageIntro, Screen } from "../../src/components";
import { useAuth } from "../../src/auth";
import { colors } from "../../src/theme";

const modules = {
  teacher: [
    ["Classes", "academic_management", "/(app)/classes"],
    ["Attendance", "attendance", "/(app)/attendance"],
    ["Assessments", "assessments", "/(app)/scores"],
    ["AI guidance review", "ai_recommendations", "/(app)/review-recommendations"],
    ["Reports", "report_cards", "/(app)/report-cards"],
  ],
  parent: [
    ["Children", "student_management", "/(app)/children"],
    ["Attendance", "attendance", "/(app)/attendance"],
    ["Results", "assessments", "/(app)/results"],
    ["Fees", "fees", "/(app)/fees"],
    ["Report cards", "report_cards", "/(app)/report-cards"],
    ["Career guidance", "career_guidance", "/(app)/career-guidance"],
  ],
  student: [
    ["Timetable", "timetable", "/(app)/timetable"],
    ["Assignments", "assignments", "/(app)/assignments"],
    ["Results", "assessments", "/(app)/results"],
    ["Report cards", "report_cards", "/(app)/report-cards"],
    ["CBT exams", "cbt_exams", "/(app)/cbt-exams"],
    ["Recommendations", "ai_recommendations", "/(app)/recommendations"],
    ["Career guidance", "career_guidance", "/(app)/career-guidance"],
  ],
} as const;

export default function Work() {
  const { session, features } = useAuth();
  const role = session?.user.role ?? "student";
  return (
    <Screen>
      <ScrollView contentContainerStyle={styles.content}>
        <PageIntro
          eyebrow="Your school day"
          title="My work"
          copy="Focused tools for your role, arranged around what needs attention now."
        />
        {modules[role].map(([title, flag, href]) => (
          <ModuleCard
            key={flag}
            title={title}
            copy={`Open your ${title.toLowerCase()} workspace.`}
            enabled={features.has(flag)}
            href={href ?? undefined}
          />
        ))}
      </ScrollView>
    </Screen>
  );
}

const styles = StyleSheet.create({
  content: { gap: 13, paddingBottom: 32 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  copy: { color: colors.muted, lineHeight: 21, marginBottom: 8 },
});
