import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  AccessibilityInfo,
  Animated,
  Easing,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { Link, type Href } from "expo-router";
import { colors, useTheme } from "./theme";

export function Screen({ children }: { children: React.ReactNode }) {
  const reduceMotion = useReduceMotion();
  const entrance = useRef(new Animated.Value(0)).current;
  const atmosphere = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    if (reduceMotion) {
      entrance.setValue(1);
      atmosphere.setValue(0);
      return;
    }

    const enter = Animated.timing(entrance, {
      toValue: 1,
      duration: 520,
      easing: Easing.out(Easing.cubic),
      useNativeDriver: true,
    });
    const drift = Animated.loop(
      Animated.sequence([
        Animated.timing(atmosphere, {
          toValue: 1,
          duration: 6500,
          easing: Easing.inOut(Easing.sin),
          useNativeDriver: true,
        }),
        Animated.timing(atmosphere, {
          toValue: 0,
          duration: 6500,
          easing: Easing.inOut(Easing.sin),
          useNativeDriver: true,
        }),
      ]),
    );
    enter.start();
    drift.start();
    return () => {
      enter.stop();
      drift.stop();
    };
  }, [atmosphere, entrance, reduceMotion]);

  const entranceStyle = reduceMotion
    ? undefined
    : {
        opacity: entrance,
        transform: [
          {
            translateY: entrance.interpolate({
              inputRange: [0, 1],
              outputRange: [12, 0],
            }),
          },
        ],
      };
  const topAtmosphereStyle = reduceMotion
    ? undefined
    : {
        transform: [
          {
            translateY: atmosphere.interpolate({
              inputRange: [0, 1],
              outputRange: [0, 14],
            }),
          },
          {
            scale: atmosphere.interpolate({
              inputRange: [0, 1],
              outputRange: [1, 1.05],
            }),
          },
        ],
      };
  const bottomAtmosphereStyle = reduceMotion
    ? undefined
    : {
        transform: [
          {
            translateY: atmosphere.interpolate({
              inputRange: [0, 1],
              outputRange: [0, -10],
            }),
          },
        ],
      };

  return (
    <View style={styles.screen}>
      <Animated.View
        pointerEvents="none"
        style={[styles.ambient, styles.ambientTop, topAtmosphereStyle]}
      />
      <Animated.View
        pointerEvents="none"
        style={[styles.ambient, styles.ambientBottom, bottomAtmosphereStyle]}
      />
      <Animated.View style={[styles.screenContent, entranceStyle]}>{children}</Animated.View>
    </View>
  );
}

function useReduceMotion() {
  const [reduceMotion, setReduceMotion] = useState(false);

  useEffect(() => {
    let mounted = true;
    void AccessibilityInfo.isReduceMotionEnabled().then((enabled) => {
      if (mounted) setReduceMotion(enabled);
    });
    const subscription = AccessibilityInfo.addEventListener("reduceMotionChanged", setReduceMotion);
    return () => {
      mounted = false;
      subscription.remove();
    };
  }, []);

  return reduceMotion;
}

export function Eyebrow({ children }: { children: React.ReactNode }) {
  const theme = useTheme();
  return <Text style={[styles.eyebrow, { color: theme.brand }]}>{children}</Text>;
}

export function PageIntro({
  eyebrow,
  title,
  copy,
}: {
  eyebrow?: string;
  title: string;
  copy?: string;
}) {
  return (
    <View style={styles.pageIntro}>
      {eyebrow ? <Eyebrow>{eyebrow}</Eyebrow> : null}
      <Text accessibilityRole="header" style={styles.pageTitle}>
        {title}
      </Text>
      {copy ? <Text style={styles.pageCopy}>{copy}</Text> : null}
      <View pointerEvents="none" style={styles.signalLine} />
    </View>
  );
}

export function ModuleCard({
  title,
  copy,
  enabled = true,
  href,
}: {
  title: string;
  copy: string;
  enabled?: boolean;
  href?: Href;
}) {
  const content = (
    <View
      accessibilityState={{ disabled: !enabled }}
      style={[styles.card, !enabled && styles.disabled]}
    >
      <View
        pointerEvents="none"
        style={[styles.cardGlow, { backgroundColor: enabled ? colors.brandSoft : colors.mist }]}
      />
      <View style={styles.cardTopline}>
        <View
          style={[
            styles.moduleMark,
            { backgroundColor: enabled ? colors.midnight : colors.border },
          ]}
        >
          <Text style={styles.moduleMarkText}>{title.slice(0, 1).toUpperCase()}</Text>
        </View>
        <View style={styles.cardText}>
          <Text style={styles.cardTitle}>{title}</Text>
          <Text style={styles.cardCopy}>{enabled ? copy : "Not enabled for this school."}</Text>
        </View>
      </View>
      {enabled && href ? (
        <View style={styles.openRow}>
          <Text style={styles.open}>Open workspace</Text>
          <View accessibilityElementsHidden style={styles.openDot} />
        </View>
      ) : null}
    </View>
  );
  return enabled && href ? (
    <Link href={href} asChild>
      <Pressable
        accessibilityRole="link"
        accessibilityLabel={`Open ${title}`}
        hitSlop={4}
        style={({ pressed }) => pressed && styles.cardPressed}
      >
        {content}
      </Pressable>
    </Link>
  ) : (
    content
  );
}

export function PrimaryButton({
  label,
  onPress,
  disabled,
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
}) {
  const theme = useTheme();
  const dynamic = useMemo(() => ({ backgroundColor: theme.brand }), [theme.brand]);
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: Boolean(disabled) }}
      disabled={disabled}
      hitSlop={4}
      onPress={onPress}
      style={({ pressed }) => [
        styles.button,
        dynamic,
        pressed && styles.pressed,
        disabled && styles.disabled,
      ]}
    >
      <Text style={[styles.buttonText, { color: theme.onBrand }]}>{label}</Text>
    </Pressable>
  );
}

export function LoadingState({ label = "Loading" }: { label?: string }) {
  return (
    <View
      accessibilityLabel={label}
      accessibilityLiveRegion="polite"
      accessibilityRole="progressbar"
      style={styles.loading}
    >
      <View aria-hidden style={[styles.loadingLine, styles.loadingLineWide]} />
      <View aria-hidden style={[styles.loadingLine, styles.loadingLineMedium]} />
      <View aria-hidden style={[styles.loadingLine, styles.loadingLineShort]} />
      <Text style={styles.loadingLabel}>{label}…</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: colors.paper,
    paddingHorizontal: 20,
    paddingTop: 18,
    paddingBottom: 88,
  },
  screenContent: { flex: 1 },
  ambient: { position: "absolute", borderRadius: 999 },
  ambientTop: {
    width: 270,
    height: 270,
    right: -142,
    top: -118,
    backgroundColor: colors.sky,
    opacity: 0.82,
  },
  ambientBottom: {
    width: 230,
    height: 230,
    left: -155,
    bottom: 36,
    backgroundColor: "#DFF4F2",
    opacity: 0.72,
  },
  eyebrow: {
    color: colors.brand,
    fontSize: 12,
    fontWeight: "800",
    letterSpacing: 1.4,
    textTransform: "uppercase",
  },
  pageIntro: { gap: 7, marginBottom: 14, paddingTop: 4 },
  pageTitle: {
    color: colors.ink,
    fontSize: 32,
    lineHeight: 37,
    fontWeight: "900",
    letterSpacing: -1.15,
  },
  pageCopy: { color: colors.muted, fontSize: 15, lineHeight: 22, maxWidth: 440 },
  signalLine: {
    width: 72,
    height: 4,
    marginTop: 8,
    borderRadius: 99,
    backgroundColor: colors.signal,
  },
  card: {
    backgroundColor: colors.surface,
    borderColor: colors.border,
    borderRadius: 18,
    borderWidth: 1,
    padding: 18,
    gap: 14,
    overflow: "hidden",
    shadowColor: colors.midnight,
    shadowOpacity: 0.07,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 8 },
    elevation: 2,
  },
  cardGlow: {
    position: "absolute",
    width: 90,
    height: 90,
    borderRadius: 99,
    right: -35,
    top: -42,
    opacity: 0.82,
  },
  cardTopline: { flexDirection: "row", alignItems: "flex-start", gap: 13 },
  moduleMark: {
    width: 38,
    height: 38,
    borderRadius: 13,
    alignItems: "center",
    justifyContent: "center",
  },
  moduleMarkText: { color: "#FFFFFF", fontSize: 14, fontWeight: "900" },
  cardText: { flex: 1, gap: 5 },
  cardTitle: { color: colors.ink, fontSize: 17, fontWeight: "800" },
  cardCopy: { color: colors.muted, fontSize: 14, lineHeight: 20 },
  openRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    borderTopColor: colors.border,
    borderTopWidth: 1,
    paddingTop: 12,
  },
  open: { color: colors.brand, fontSize: 12, fontWeight: "900", letterSpacing: 0.3 },
  openDot: { width: 8, height: 8, borderRadius: 4, backgroundColor: colors.signal },
  cardPressed: { opacity: 0.88, transform: [{ scale: 0.985 }] },
  button: {
    minHeight: 50,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: 16,
    paddingHorizontal: 18,
    shadowColor: colors.cobalt,
    shadowOpacity: 0.2,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 8 },
    elevation: 3,
  },
  buttonText: { color: colors.paper, fontSize: 15, fontWeight: "800" },
  pressed: { opacity: 0.82 },
  disabled: { opacity: 0.48 },
  loading: { gap: 8, paddingVertical: 8 },
  loadingLine: { height: 10, borderRadius: 999, backgroundColor: colors.border },
  loadingLineWide: { width: "100%" },
  loadingLineMedium: { width: "76%" },
  loadingLineShort: { width: "48%" },
  loadingLabel: { color: colors.muted, fontSize: 12, fontWeight: "700" },
});
