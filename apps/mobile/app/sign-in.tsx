import { router } from "expo-router";
import { useState } from "react";
import {
  Image,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { StatusBar } from "expo-status-bar";
import { PrimaryButton } from "../src/components";
import { useAuth } from "../src/auth";
import { colors } from "../src/theme";
import auraEduLogo from "../assets/auraedu-logo-light.png";

export default function SignIn() {
  const { tenantCode, signIn } = useAuth();
  const [tenant, setTenant] = useState(tenantCode);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError("");
    try {
      await signIn({ tenantCode: tenant, email, password });
      router.replace("/(app)");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Sign-in failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === "ios" ? "padding" : undefined}
      style={styles.screen}
    >
      <StatusBar style="light" />
      <View pointerEvents="none" style={styles.auroraOne} />
      <View pointerEvents="none" style={styles.auroraTwo} />
      <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
        <View style={styles.hero}>
          <Image
            accessibilityLabel="AuraEDU"
            resizeMode="contain"
            source={auraEduLogo}
            style={styles.logo}
          />
          <View style={styles.pill}>
            <View style={styles.liveDot} />
            <Text style={styles.pillText}>SECURE EDUCATION OS</Text>
          </View>
          <Text style={styles.eyebrow}>YOUR SCHOOL · YOUR ROLE</Text>
          <Text accessibilityRole="header" style={styles.title}>
            One focused place for the school day.
          </Text>
          <Text style={styles.copy}>
            Teachers, parents and students use the same secure app with different role-aware
            workspaces.
          </Text>
        </View>
        <View style={styles.form}>
          <View>
            <Text style={styles.formTitle}>Enter your workspace</Text>
            <Text style={styles.formCopy}>Use the details provided by your school.</Text>
          </View>
          <Field
            label="School code"
            value={tenant}
            onChangeText={setTenant}
            autoCapitalize="none"
            autoCorrect={false}
            returnKeyType="next"
          />
          <Field
            label="Email"
            value={email}
            onChangeText={setEmail}
            autoCapitalize="none"
            autoComplete="email"
            autoCorrect={false}
            keyboardType="email-address"
            returnKeyType="next"
            textContentType="emailAddress"
          />
          <Field
            label="Password"
            value={password}
            onChangeText={setPassword}
            autoCapitalize="none"
            autoComplete="current-password"
            returnKeyType="done"
            secureTextEntry
            textContentType="password"
            onSubmitEditing={() => {
              if (!busy && tenant && email && password) void submit();
            }}
          />
          {error ? (
            <Text accessibilityRole="alert" style={styles.error}>
              {error}
            </Text>
          ) : null}
          <PrimaryButton
            label={busy ? "Signing in…" : "Sign in securely"}
            disabled={busy || !tenant || !email || !password}
            onPress={() => void submit()}
          />
          <Text style={styles.note}>School administrators use the AuraEDU web console.</Text>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

function Field(props: React.ComponentProps<typeof TextInput> & { label: string }) {
  const { label, ...input } = props;
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      <TextInput
        accessibilityLabel={label}
        placeholderTextColor="#8A958E"
        style={styles.input}
        {...input}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: colors.midnight,
    paddingHorizontal: 20,
  },
  content: { flexGrow: 1, justifyContent: "center", gap: 24, paddingVertical: 42 },
  auroraOne: {
    position: "absolute",
    width: 340,
    height: 340,
    borderRadius: 180,
    right: -170,
    top: -120,
    backgroundColor: colors.cobalt,
    opacity: 0.34,
  },
  auroraTwo: {
    position: "absolute",
    width: 260,
    height: 260,
    borderRadius: 140,
    left: -175,
    bottom: -70,
    backgroundColor: colors.teal,
    opacity: 0.28,
  },
  hero: { gap: 12 },
  logo: { width: 190, height: 44, marginBottom: 8 },
  pill: {
    alignSelf: "flex-start",
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    paddingHorizontal: 11,
    paddingVertical: 7,
    borderRadius: 99,
    backgroundColor: "rgba(255,255,255,0.08)",
    borderWidth: 1,
    borderColor: "rgba(255,255,255,0.12)",
  },
  liveDot: { width: 7, height: 7, borderRadius: 5, backgroundColor: colors.signal },
  pillText: { color: "#FFFFFF", fontSize: 9, fontWeight: "900", letterSpacing: 1.2 },
  eyebrow: { color: colors.tealBright, fontSize: 11, fontWeight: "900", letterSpacing: 1.3 },
  title: {
    color: "#FFFFFF",
    fontSize: 36,
    lineHeight: 40,
    fontWeight: "900",
    letterSpacing: -1.2,
  },
  copy: { color: colors.ink200, fontSize: 16, lineHeight: 24 },
  form: {
    backgroundColor: colors.surface,
    borderColor: colors.border,
    borderWidth: 1,
    borderRadius: 22,
    padding: 22,
    gap: 16,
    shadowColor: colors.midnight,
    shadowOpacity: 0.24,
    shadowRadius: 24,
    shadowOffset: { width: 0, height: 10 },
    elevation: 4,
  },
  formTitle: { color: colors.ink, fontSize: 20, fontWeight: "900" },
  formCopy: { color: colors.muted, fontSize: 13, marginTop: 4 },
  field: { gap: 7 },
  label: { color: colors.ink, fontSize: 13, fontWeight: "700" },
  input: {
    minHeight: 50,
    borderColor: colors.border,
    borderWidth: 1,
    borderRadius: 14,
    paddingHorizontal: 14,
    color: colors.ink,
    fontSize: 16,
    backgroundColor: colors.paper,
  },
  error: { color: colors.danger, lineHeight: 20 },
  note: { color: colors.muted, fontSize: 12, textAlign: "center" },
});
