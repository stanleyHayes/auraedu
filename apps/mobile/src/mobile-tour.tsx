import AsyncStorage from "@react-native-async-storage/async-storage";
import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AccessibilityInfo,
  Animated,
  DeviceEventEmitter,
  Dimensions,
  Modal,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { useAuth } from "./auth";
import { colors } from "./theme";

const replayEvent = "auraedu:replay-mobile-tour";

export function replayMobileTour() {
  DeviceEventEmitter.emit(replayEvent);
}

const tourSteps = [
  {
    title: "Today",
    copy: "Start with a role-aware brief of the school work that needs your attention.",
    tab: 0,
  },
  {
    title: "My work",
    copy: "Open only the modules enabled for your school and permitted for your role.",
    tab: 1,
  },
  {
    title: "Notices",
    copy: "Keep tenant-scoped announcements and optional push alerts in one calm inbox.",
    tab: 2,
  },
  {
    title: "Profile",
    copy: "Confirm your active school, replay this tour or securely sign out from this device.",
    tab: 3,
  },
];

export function MobileTour() {
  const { ready, session } = useAuth();
  const [open, setOpen] = useState(false);
  const [step, setStep] = useState(0);
  const [reduceMotion, setReduceMotion] = useState(false);
  const entrance = useRef(new Animated.Value(0)).current;
  const key = session
    ? `auraedu-tour-complete:${session.user.role}:${session.user.tenant_id}:${session.user.id}`
    : "";

  const finish = useCallback(() => {
    if (key) void AsyncStorage.setItem(key, "1");
    setOpen(false);
  }, [key]);

  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const motion = AccessibilityInfo.addEventListener("reduceMotionChanged", setReduceMotion);
    const replay = DeviceEventEmitter.addListener(replayEvent, () => {
      setStep(0);
      setOpen(true);
    });
    return () => {
      motion.remove();
      replay.remove();
    };
  }, []);

  useEffect(() => {
    if (!ready || !session || !key) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    void AsyncStorage.getItem(key).then((complete) => {
      if (!active || complete) return;
      timer = setTimeout(() => {
        if (active) setOpen(true);
      }, 650);
    });
    return () => {
      active = false;
      if (timer) clearTimeout(timer);
    };
  }, [key, ready, session]);

  useEffect(() => {
    if (!open || reduceMotion) {
      entrance.setValue(1);
      return;
    }
    entrance.setValue(0);
    Animated.spring(entrance, {
      toValue: 1,
      damping: 18,
      stiffness: 180,
      useNativeDriver: true,
    }).start();
  }, [entrance, open, reduceMotion, step]);

  const tabWidth = (Dimensions.get("window").width - 28) / 4;
  const current = tourSteps[step]!;
  const highlight = useMemo(
    () => ({ left: 14 + current.tab * tabWidth + 4, width: tabWidth - 8 }),
    [current.tab, tabWidth],
  );

  if (!session) return null;

  return (
    <Modal
      visible={open}
      transparent
      animationType="none"
      statusBarTranslucent
      onRequestClose={finish}
    >
      <View accessibilityViewIsModal style={styles.overlay}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Close tour and do not show again"
          onPress={finish}
          style={StyleSheet.absoluteFill}
        />
        <View pointerEvents="none" style={[styles.tabHighlight, highlight]} />
        <Animated.View
          accessibilityRole="summary"
          accessibilityLabel={`${current.title}. ${current.copy}`}
          style={[
            styles.card,
            reduceMotion
              ? undefined
              : {
                  opacity: entrance,
                  transform: [
                    {
                      translateY: entrance.interpolate({
                        inputRange: [0, 1],
                        outputRange: [18, 0],
                      }),
                    },
                  ],
                },
          ]}
        >
          <View style={styles.progress}>
            {tourSteps.map((item, index) => (
              <View
                key={item.title}
                style={[styles.progressItem, index <= step && styles.progressItemActive]}
              />
            ))}
          </View>
          <Text style={styles.eyebrow}>
            Guided tour · {step + 1} of {tourSteps.length}
          </Text>
          <Text accessibilityRole="header" style={styles.title}>
            {current.title}
          </Text>
          <Text style={styles.copy}>{current.copy}</Text>
          <View style={styles.actions}>
            <Pressable
              accessibilityRole="button"
              accessibilityState={{ disabled: step === 0 }}
              disabled={step === 0}
              onPress={() => setStep((value) => Math.max(0, value - 1))}
              style={[styles.secondary, step === 0 && styles.disabled]}
            >
              <Text style={styles.secondaryText}>Back</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              onPress={() =>
                step === tourSteps.length - 1 ? finish() : setStep((value) => value + 1)
              }
              style={styles.primary}
            >
              <Text style={styles.primaryText}>
                {step === tourSteps.length - 1 ? "Finish" : "Next"}
              </Text>
            </Pressable>
          </View>
        </Animated.View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    justifyContent: "flex-end",
    backgroundColor: "rgba(4, 6, 15, 0.68)",
    paddingHorizontal: 18,
    paddingBottom: 96,
  },
  tabHighlight: {
    position: "absolute",
    bottom: 8,
    height: 74,
    borderRadius: 20,
    borderWidth: 2,
    borderColor: colors.signal,
    backgroundColor: "rgba(183,245,0,0.10)",
  },
  card: {
    borderRadius: 24,
    backgroundColor: colors.midnight,
    borderWidth: 1,
    borderColor: "rgba(255,255,255,0.14)",
    padding: 22,
    shadowColor: "#000",
    shadowOpacity: 0.28,
    shadowRadius: 24,
    shadowOffset: { width: 0, height: 14 },
    elevation: 14,
  },
  progress: { flexDirection: "row", gap: 6, marginBottom: 18 },
  progressItem: { height: 3, flex: 1, borderRadius: 9, backgroundColor: "rgba(255,255,255,0.16)" },
  progressItemActive: { backgroundColor: colors.signal },
  eyebrow: {
    color: colors.teal,
    fontSize: 11,
    fontWeight: "900",
    letterSpacing: 1.3,
    textTransform: "uppercase",
  },
  title: { color: "#FFFFFF", fontSize: 27, lineHeight: 33, fontWeight: "900", marginTop: 6 },
  copy: { color: "rgba(255,255,255,0.70)", fontSize: 15, lineHeight: 22, marginTop: 8 },
  actions: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginTop: 22,
  },
  secondary: { minHeight: 44, justifyContent: "center", paddingHorizontal: 16 },
  secondaryText: { color: "rgba(255,255,255,0.72)", fontWeight: "800" },
  primary: {
    minHeight: 46,
    minWidth: 104,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: 15,
    backgroundColor: colors.signal,
    paddingHorizontal: 20,
  },
  primaryText: { color: colors.midnight, fontWeight: "900" },
  disabled: { opacity: 0.3 },
});
