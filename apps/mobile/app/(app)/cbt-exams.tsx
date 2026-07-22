import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import {
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Exam {
  id: string;
  title: string;
  duration_minutes: number;
  question_ids: string[];
  start_at?: string | null;
  end_at?: string | null;
  status: string;
}
interface Submission {
  id: string;
  status: string;
  score?: number | null;
  max_score: number;
}
interface Question {
  id: string;
  question_text: string;
  question_type: "multiple_choice" | "true_false" | "short_answer";
  options?: string[];
  marks: number;
}

export default function StudentCBTExams() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const enabled = features.has("cbt_exams");
  const [exams, setExams] = useState<Exam[]>([]);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [submission, setSubmission] = useState<Submission | null>(null);
  const [activeTitle, setActiveTitle] = useState("");
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
      const body = await client.get<{ data?: Exam[] }>("/api/v1/cbt/exams?limit=100");
      setExams(body.data ?? []);
    } catch {
      setError("CBT exams could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, featuresReady]);
  useFocusEffect(
    useCallback(() => {
      if (!submission) void load();
    }, [load, submission]),
  );
  const start = async (exam: Exam) => {
    if (!client) return;
    setLoading(true);
    setError("");
    try {
      const attempt = await client.post<Submission>(`/api/v1/cbt/exams/${exam.id}/start`, {});
      const body = await client.get<{ data?: Question[] }>(
        `/api/v1/cbt/submissions/${attempt.id}/questions`,
      );
      setSubmission(attempt);
      setQuestions(body.data ?? []);
      setAnswers({});
      setActiveTitle(exam.title);
    } catch {
      setError("This exam could not be started. It may no longer be active.");
    } finally {
      setLoading(false);
    }
  };
  const submit = async () => {
    if (!client || !submission) return;
    setLoading(true);
    setError("");
    try {
      setSubmission(
        await client.post<Submission>(`/api/v1/cbt/submissions/${submission.id}/submit`, {
          answers,
        }),
      );
    } catch {
      setError("Your answers could not be submitted. Check your connection and try again.");
    } finally {
      setLoading(false);
    }
  };
  if (session?.user.role !== "student") return <Redirect href="/(app)" />;
  // Explicit guard also narrows submission for the result view below.
  // eslint-disable-next-line @typescript-eslint/prefer-optional-chain
  if (submission && submission.status === "graded")
    return (
      <Screen>
        <ScrollView contentContainerStyle={styles.content}>
          <PageIntro eyebrow="Submission received" title="Exam complete" />
          <View style={styles.result}>
            <Text style={styles.resultScore}>
              {submission.score ?? 0} / {submission.max_score}
            </Text>
            <Text style={styles.copy}>Your answers were submitted and graded securely.</Text>
            <TouchableOpacity
              accessibilityRole="button"
              style={[styles.button, { backgroundColor: theme.brand }]}
              onPress={() => {
                setSubmission(null);
                setQuestions([]);
                void load();
              }}
            >
              <Text style={styles.buttonText}>Back to exams</Text>
            </TouchableOpacity>
          </View>
        </ScrollView>
      </Screen>
    );
  if (submission)
    return (
      <Screen>
        <ScrollView contentContainerStyle={styles.content}>
          <PageIntro
            eyebrow="Active assessment"
            title={activeTitle}
            copy="Answer every question you can. Your final submission cannot be edited."
          />
          {error ? <State title="Could not submit" copy={error} /> : null}
          {questions.map((question, index) => (
            <View key={question.id} style={styles.card}>
              <Text style={styles.question}>
                {index + 1}. {question.question_text}
              </Text>
              <Text style={styles.marks}>
                {question.marks} mark{question.marks === 1 ? "" : "s"}
              </Text>
              {question.question_type === "short_answer" ? (
                <TextInput
                  accessibilityLabel={`Answer to question ${index + 1}`}
                  value={answers[question.id] ?? ""}
                  onChangeText={(value) =>
                    setAnswers((current) => ({ ...current, [question.id]: value }))
                  }
                  placeholder="Your answer"
                  style={styles.input}
                />
              ) : (
                (
                  question.options ??
                  (question.question_type === "true_false" ? ["true", "false"] : [])
                ).map((option) => (
                  <TouchableOpacity
                    key={option}
                    accessibilityLabel={`${option}, question ${index + 1}`}
                    accessibilityRole="radio"
                    accessibilityState={{ checked: answers[question.id] === option }}
                    onPress={() => setAnswers((current) => ({ ...current, [question.id]: option }))}
                    style={[
                      styles.option,
                      answers[question.id] === option && {
                        borderColor: theme.brand,
                        backgroundColor: `${theme.brand}18`,
                      },
                    ]}
                  >
                    <Text style={styles.optionText}>{option}</Text>
                  </TouchableOpacity>
                ))
              )}
            </View>
          ))}
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityState={{ disabled: loading }}
            disabled={loading}
            style={[styles.button, { backgroundColor: theme.brand }, loading && styles.disabled]}
            onPress={() => void submit()}
          >
            <Text style={styles.buttonText}>
              {loading ? "Submitting…" : "Submit final answers"}
            </Text>
          </TouchableOpacity>
        </ScrollView>
      </Screen>
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
          eyebrow="Secure assessment"
          title="CBT exams"
          copy="Only exams currently open for your school are shown."
        />
        {loading ? <LoadingState label="Loading active exams" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="CBT exams are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && exams.length === 0 ? (
          <State
            title="No active exams"
            copy="Scheduled online exams will appear here when they open."
          />
        ) : null}
        {!loading && !error
          ? exams.map((exam) => (
              <View key={exam.id} style={styles.card}>
                <Text style={styles.examTitle}>{exam.title}</Text>
                <Text style={styles.copy}>
                  {exam.question_ids.length} questions · {exam.duration_minutes} minutes
                </Text>
                <TouchableOpacity
                  accessibilityLabel={`Start ${exam.title}`}
                  accessibilityRole="button"
                  style={[styles.button, { backgroundColor: theme.brand }]}
                  onPress={() => void start(exam)}
                >
                  <Text style={styles.buttonText}>Start exam</Text>
                </TouchableOpacity>
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
  content: { gap: 13, paddingBottom: 40 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  copy: { color: colors.muted, lineHeight: 21 },
  card: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 17,
    gap: 10,
  },
  examTitle: { color: colors.ink, fontSize: 18, fontWeight: "900" },
  question: { color: colors.ink, fontSize: 16, fontWeight: "800", lineHeight: 23 },
  marks: { color: colors.muted, fontSize: 12, fontWeight: "700" },
  input: {
    minHeight: 48,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingHorizontal: 13,
    color: colors.ink,
    backgroundColor: colors.paper,
  },
  option: { borderWidth: 1, borderColor: colors.border, borderRadius: 12, padding: 13 },
  optionText: { color: colors.ink, fontWeight: "700" },
  button: { borderRadius: 12, paddingHorizontal: 16, paddingVertical: 13, alignItems: "center" },
  buttonText: { color: "#16210B", fontWeight: "900" },
  disabled: { opacity: 0.55 },
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
  result: {
    borderRadius: 20,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 28,
    alignItems: "center",
    gap: 16,
  },
  resultScore: { color: colors.ink, fontSize: 42, fontWeight: "900" },
});
