import { StatusBar } from "expo-status-bar";
import { StyleSheet, Text, View } from "react-native";

export default function App(): JSX.Element {
  return (
    <View style={styles.container}>
      <Text style={styles.title}>AuraEDU</Text>
      <Text style={styles.subtitle}>Teacher / Parent / Student Portal</Text>
      <StatusBar style="auto" />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#fff",
    alignItems: "center",
    justifyContent: "center",
    padding: 24,
  },
  title: {
    fontSize: 32,
    fontWeight: "700",
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 16,
    color: "#555",
    textAlign: "center",
  },
});
