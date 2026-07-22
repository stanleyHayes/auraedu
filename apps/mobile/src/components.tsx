import React, { useEffect, useState } from "react";
import { Modal, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import * as Speech from "expo-speech";
import {
  Eyebrow,
  LoadingState,
  ModuleCard,
  PageIntro as BasePageIntro,
  PrimaryButton,
  Screen,
} from "@auraedu/ui-native";
import { getMobileGuide } from "./mobile-guides";
import { colors } from "./theme";

export { Eyebrow, LoadingState, ModuleCard, PrimaryButton, Screen };

export function PageIntro({
  eyebrow,
  title,
  copy,
}: {
  eyebrow?: string;
  title: string;
  copy?: string;
}) {
  const [open, setOpen] = useState(false);
  const [speaking, setSpeaking] = useState(false);
  const guide = getMobileGuide(title, copy);

  useEffect(
    () => () => {
      void Speech.stop();
    },
    [],
  );

  function close() {
    void Speech.stop();
    setSpeaking(false);
    setOpen(false);
  }

  function toggleSpeech() {
    if (speaking) {
      void Speech.stop();
      setSpeaking(false);
      return;
    }
    setSpeaking(true);
    Speech.speak([guide.title, guide.description, ...guide.steps].join(". "), {
      language: "en-GB",
      onDone: () => setSpeaking(false),
      onStopped: () => setSpeaking(false),
      onError: () => setSpeaking(false),
    });
  }

  return (
    <View style={styles.introWrap}>
      <BasePageIntro eyebrow={eyebrow} title={title} copy={copy} />
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={`How to use ${title}`}
        hitSlop={6}
        onPress={() => setOpen(true)}
        style={({ pressed }) => [styles.helpButton, pressed && styles.pressed]}
      >
        <Text style={styles.helpGlyph}>?</Text>
      </Pressable>
      <Modal visible={open} transparent animationType="fade" onRequestClose={close}>
        <View accessibilityViewIsModal style={styles.guideOverlay}>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Close page guide"
            onPress={close}
            style={StyleSheet.absoluteFill}
          />
          <View style={styles.guideSheet}>
            <View style={styles.guideSignal} />
            <View style={styles.guideHeader}>
              <View style={styles.guideHeading}>
                <Text style={styles.guideEyebrow}>Page guide</Text>
                <Text accessibilityRole="header" style={styles.guideTitle}>
                  {guide.title}
                </Text>
              </View>
              <Pressable
                accessibilityRole="button"
                accessibilityLabel="Close page guide"
                hitSlop={8}
                onPress={close}
                style={styles.closeButton}
              >
                <Text style={styles.closeGlyph}>×</Text>
              </Pressable>
            </View>
            <Text style={styles.guideDescription}>{guide.description}</Text>
            <ScrollView style={styles.guideSteps} contentContainerStyle={styles.guideStepsContent}>
              {guide.steps.map((step, index) => (
                <View key={step} style={styles.guideStep}>
                  <View style={styles.stepNumber}>
                    <Text style={styles.stepNumberText}>{index + 1}</Text>
                  </View>
                  <Text style={styles.stepText}>{step}</Text>
                </View>
              ))}
            </ScrollView>
            <View style={styles.guideFooter}>
              <Text style={styles.voiceNote}>British English narration</Text>
              <Pressable
                accessibilityRole="button"
                accessibilityState={{ selected: speaking }}
                accessibilityLabel={speaking ? "Stop page guide narration" : "Listen to page guide"}
                onPress={toggleSpeech}
                style={styles.listenButton}
              >
                <Text style={styles.listenText}>{speaking ? "■  Stop" : "▶  Listen"}</Text>
              </Pressable>
            </View>
          </View>
        </View>
      </Modal>
    </View>
  );
}

const styles = StyleSheet.create({
  introWrap: { position: "relative" },
  helpButton: {
    position: "absolute",
    right: 0,
    top: 2,
    width: 42,
    height: 42,
    borderRadius: 21,
    alignItems: "center",
    justifyContent: "center",
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
  },
  helpGlyph: { color: colors.cobalt, fontSize: 18, fontWeight: "900" },
  pressed: { opacity: 0.72, transform: [{ scale: 0.97 }] },
  guideOverlay: { flex: 1, justifyContent: "flex-end", backgroundColor: "rgba(4,6,15,0.62)" },
  guideSheet: {
    maxHeight: "82%",
    borderTopLeftRadius: 28,
    borderTopRightRadius: 28,
    backgroundColor: colors.surface,
    paddingHorizontal: 22,
    paddingBottom: 30,
    overflow: "hidden",
  },
  guideSignal: { height: 4, marginHorizontal: -22, backgroundColor: colors.signal },
  guideHeader: {
    flexDirection: "row",
    alignItems: "flex-start",
    justifyContent: "space-between",
    gap: 16,
    marginTop: 22,
  },
  guideHeading: { flex: 1 },
  guideEyebrow: {
    color: colors.cobalt,
    fontSize: 11,
    fontWeight: "900",
    letterSpacing: 1.4,
    textTransform: "uppercase",
  },
  guideTitle: { color: colors.ink, fontSize: 26, lineHeight: 32, fontWeight: "900", marginTop: 4 },
  closeButton: {
    width: 42,
    height: 42,
    borderRadius: 21,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: colors.mist,
  },
  closeGlyph: { color: colors.ink, fontSize: 28, lineHeight: 29, fontWeight: "500" },
  guideDescription: { color: colors.muted, fontSize: 15, lineHeight: 22, marginTop: 8 },
  guideSteps: { marginTop: 18 },
  guideStepsContent: { gap: 14, paddingBottom: 6 },
  guideStep: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  stepNumber: {
    width: 30,
    height: 30,
    borderRadius: 15,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: colors.brandSoft,
  },
  stepNumberText: { color: colors.cobalt, fontSize: 12, fontWeight: "900" },
  stepText: { flex: 1, color: colors.ink, fontSize: 15, lineHeight: 22, paddingTop: 3 },
  guideFooter: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 14,
    borderTopWidth: 1,
    borderTopColor: colors.border,
    marginTop: 20,
    paddingTop: 18,
  },
  voiceNote: { flex: 1, color: colors.muted, fontSize: 12 },
  listenButton: {
    minHeight: 46,
    minWidth: 108,
    borderRadius: 15,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: colors.cobalt,
    paddingHorizontal: 16,
  },
  listenText: { color: "#FFFFFF", fontSize: 13, fontWeight: "900" },
});
