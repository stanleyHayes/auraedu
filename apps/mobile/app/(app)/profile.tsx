import { router } from "expo-router";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { PageIntro, PrimaryButton, Screen } from "../../src/components";
import { useAuth } from "../../src/auth";
import { colors } from "../../src/theme";
import { replayMobileTour } from "../../src/mobile-tour";

export default function Profile() {
  const { session, tenantCode, signOut } = useAuth();
  async function leave() {
    await signOut();
    router.replace("/sign-in");
  }
  return (
    <Screen>
      <PageIntro
        eyebrow="Account & access"
        title="Profile"
        copy="Your secure identity across this school workspace."
      />
      <View style={styles.card}>
        <Text style={styles.name}>{session?.user.name}</Text>
        <Text style={styles.copy}>{session?.user.email}</Text>
        <Text style={styles.badge}>
          {tenantCode} · {session?.user.role}
        </Text>
      </View>
      <Pressable
        accessibilityRole="button"
        onPress={replayMobileTour}
        style={({ pressed }) => [styles.tourButton, pressed && styles.pressed]}
      >
        <Text style={styles.tourButtonTitle}>Show me around</Text>
        <Text style={styles.tourButtonCopy}>Replay the guided mobile workspace tour</Text>
      </Pressable>
      <PrimaryButton label="Sign out" onPress={() => void leave()} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  title: { color: colors.ink, fontSize: 30, fontWeight: "900", marginBottom: 20 },
  card: {
    padding: 20,
    borderRadius: 16,
    borderColor: colors.border,
    borderWidth: 1,
    backgroundColor: colors.surface,
    gap: 7,
    marginBottom: 18,
  },
  name: { color: colors.ink, fontSize: 21, fontWeight: "900" },
  copy: { color: colors.muted },
  badge: {
    marginTop: 8,
    color: colors.brand,
    textTransform: "uppercase",
    letterSpacing: 1,
    fontSize: 11,
    fontWeight: "900",
  },
  tourButton: {
    minHeight: 62,
    justifyContent: "center",
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    paddingHorizontal: 18,
    marginBottom: 12,
  },
  tourButtonTitle: { color: colors.cobalt, fontSize: 15, fontWeight: "900" },
  tourButtonCopy: { color: colors.muted, fontSize: 12, marginTop: 3 },
  pressed: { opacity: 0.78, transform: [{ scale: 0.985 }] },
});
