import { Redirect, Tabs } from "expo-router";
import { SymbolView, type SFSymbol } from "expo-symbols";
import { type ColorValue, StyleSheet, View } from "react-native";
import { useAuth } from "../../src/auth";
import { colors } from "../../src/theme";
import { MobileTour } from "../../src/mobile-tour";

export default function AppLayout() {
  const { ready, session } = useAuth();
  if (ready && !session) return <Redirect href="/sign-in" />;
  return (
    <>
      <Tabs
        screenOptions={{
          headerStyle: { backgroundColor: colors.paper },
          headerTintColor: colors.ink,
          headerShadowVisible: false,
          headerTitleStyle: { fontWeight: "900", color: colors.ink },
          headerTitleAlign: "left",
          tabBarActiveTintColor: colors.signal,
          tabBarInactiveTintColor: colors.ink200,
          tabBarStyle: styles.tabBar,
          tabBarItemStyle: styles.tabItem,
          tabBarLabelStyle: styles.tabLabel,
          tabBarHideOnKeyboard: true,
        }}
      >
        <Tabs.Screen
          name="index"
          options={{
            title: "Today",
            tabBarAccessibilityLabel: "Today tab",
            tabBarIcon: ({ color, focused }) => (
              <TabIcon kind="today" color={color} focused={focused} />
            ),
          }}
        />
        <Tabs.Screen
          name="work"
          options={{
            title: "My work",
            tabBarAccessibilityLabel: "My work tab",
            tabBarIcon: ({ color, focused }) => (
              <TabIcon kind="work" color={color} focused={focused} />
            ),
          }}
        />
        <Tabs.Screen
          name="notifications"
          options={{
            title: "Notices",
            tabBarAccessibilityLabel: "Notices tab",
            tabBarIcon: ({ color, focused }) => (
              <TabIcon kind="notice" color={color} focused={focused} />
            ),
          }}
        />
        <Tabs.Screen
          name="profile"
          options={{
            title: "Profile",
            tabBarAccessibilityLabel: "Profile tab",
            tabBarIcon: ({ color, focused }) => (
              <TabIcon kind="profile" color={color} focused={focused} />
            ),
          }}
        />
        <Tabs.Screen name="children" options={{ href: null, title: "My children" }} />
        <Tabs.Screen name="classes" options={{ href: null, title: "My classes" }} />
        <Tabs.Screen name="attendance" options={{ href: null, title: "Attendance" }} />
        <Tabs.Screen name="results" options={{ href: null, title: "Results" }} />
        <Tabs.Screen name="scores" options={{ href: null, title: "Record scores" }} />
        <Tabs.Screen name="assignments" options={{ href: null, title: "Assignments" }} />
        <Tabs.Screen name="fees" options={{ href: null, title: "Fees" }} />
        <Tabs.Screen name="report-cards" options={{ href: null, title: "Report cards" }} />
        <Tabs.Screen name="timetable" options={{ href: null, title: "Timetable" }} />
        <Tabs.Screen name="recommendations" options={{ href: null, title: "Recommendations" }} />
        <Tabs.Screen
          name="review-recommendations"
          options={{ href: null, title: "Review guidance" }}
        />
        <Tabs.Screen name="cbt-exams" options={{ href: null, title: "CBT exams" }} />
        <Tabs.Screen name="career-guidance" options={{ href: null, title: "Career guidance" }} />
      </Tabs>
      <MobileTour />
    </>
  );
}

function TabIcon({
  kind,
  color,
  focused,
}: {
  kind: "today" | "work" | "notice" | "profile";
  color: ColorValue;
  focused: boolean;
}) {
  const symbols: Record<
    typeof kind,
    { ios: SFSymbol; android: "today" | "workspaces" | "notifications" | "person" }
  > = {
    today: { ios: "calendar", android: "today" },
    work: { ios: "square.grid.2x2", android: "workspaces" },
    notice: { ios: "bell", android: "notifications" },
    profile: { ios: "person.crop.circle", android: "person" },
  };

  return (
    <View accessible={false} style={[styles.iconPlate, focused && styles.iconPlateActive]}>
      <SymbolView
        name={symbols[kind]}
        size={19}
        tintColor={color}
        type={focused ? "hierarchical" : "monochrome"}
        weight={focused ? "bold" : "semibold"}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  tabBar: {
    position: "absolute",
    left: 14,
    right: 14,
    bottom: 10,
    height: 70,
    paddingTop: 7,
    paddingBottom: 9,
    backgroundColor: colors.midnight,
    borderTopWidth: 0,
    borderRadius: 22,
    shadowColor: colors.midnight,
    shadowOpacity: 0.22,
    shadowRadius: 24,
    shadowOffset: { width: 0, height: 10 },
    elevation: 10,
  },
  tabItem: { borderRadius: 16 },
  tabLabel: { fontSize: 10, fontWeight: "800", marginTop: 1 },
  iconPlate: {
    width: 34,
    height: 28,
    borderRadius: 11,
    alignItems: "center",
    justifyContent: "center",
  },
  iconPlateActive: { backgroundColor: "rgba(183,245,0,0.12)" },
});
