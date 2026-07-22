import { Redirect } from "expo-router";
import { View } from "react-native";
import { useAuth } from "../src/auth";
import { LoadingState } from "../src/components";
import { colors } from "../src/theme";

export default function Index() {
  const { ready, session } = useAuth();
  if (!ready)
    return (
      <View
        style={{ flex: 1, justifyContent: "center", backgroundColor: colors.paper, padding: 32 }}
      >
        <LoadingState label="Restoring your secure session" />
      </View>
    );
  return <Redirect href={session ? "/(app)" : "/sign-in"} />;
}
