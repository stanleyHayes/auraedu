import { Redirect, useFocusEffect } from "expo-router";
import React, { useCallback, useState } from "react";
import { Linking, RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { useAuth } from "../../src/auth";
import { LoadingState, PageIntro, PrimaryButton, Screen } from "../../src/components";
import { colors, useTheme } from "../../src/theme";

interface Invoice {
  id: string;
  student_id: string;
  fee_structure_id: string;
  amount_cents: number;
  balance_cents: number;
  status: string;
  due_date?: string;
}
interface FeeStructure {
  id: string;
  name: string;
  currency: string;
}
interface Student {
  id: string;
  first_name: string;
  last_name: string;
}
interface Payment {
  id: string;
  invoice_id: string;
  status: string;
  checkout_url?: string;
}

export default function Fees() {
  const { client, features, featuresReady, session } = useAuth();
  const theme = useTheme();
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [students, setStudents] = useState<Record<string, string>>({});
  const [structures, setStructures] = useState<Record<string, FeeStructure>>({});
  const [payments, setPayments] = useState<Record<string, Payment>>({});
  const [initiating, setInitiating] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const enabled = features.has("fees");
  const load = useCallback(async () => {
    if (!featuresReady) return;
    if (!enabled || !client || !session) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [invoiceResponse, structureResponse, family, paymentResponse] = await Promise.all([
        client.get<{ data?: Invoice[] }>("/api/v1/invoices?limit=100"),
        client.get<{ data?: FeeStructure[] }>("/api/v1/fee-structures?limit=100"),
        client.get<{ students?: Student[] }>("/api/v1/guardians/me/children"),
        features.has("online_payments")
          ? client.get<{ data?: Payment[] }>("/api/v1/payments?limit=100")
          : Promise.resolve({ data: [] }),
      ]);
      setInvoices(invoiceResponse.data ?? []);
      setStructures(
        Object.fromEntries((structureResponse.data ?? []).map((item) => [item.id, item])),
      );
      setStudents(
        Object.fromEntries(
          (family.students ?? []).map((student) => [
            student.id,
            `${student.first_name} ${student.last_name}`,
          ]),
        ),
      );
      setPayments(
        Object.fromEntries(
          (paymentResponse.data ?? []).map((payment) => [payment.invoice_id, payment]),
        ),
      );
    } catch {
      setError("Your fee account could not be refreshed.");
    } finally {
      setLoading(false);
    }
  }, [client, enabled, features, featuresReady, session]);
  const initiate = useCallback(
    async (payment: Payment) => {
      if (!client) return;
      setInitiating(payment.id);
      setError("");
      try {
        const result = await client.post<Payment>(`/api/v1/payments/${payment.id}/initiate`, {});
        if (!result.checkout_url) throw new Error("checkout unavailable");
        const checkout = new URL(result.checkout_url);
        if (checkout.protocol !== "https:") throw new Error("insecure checkout");
        if (!(await Linking.canOpenURL(checkout.toString())))
          throw new Error("checkout unsupported");
        setPayments((current) => ({ ...current, [result.invoice_id]: result }));
        await Linking.openURL(checkout.toString());
      } catch {
        setError("Secure payment checkout could not be started.");
      } finally {
        setInitiating("");
      }
    },
    [client],
  );
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );
  if (session && session.user.role !== "parent") return <Redirect href="/(app)" />;
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
          eyebrow="Family finances"
          title="Fees"
          copy="Invoices and secure payment checkout for your linked children."
        />
        {loading ? <LoadingState label="Loading fees" /> : null}
        {featuresReady && !enabled ? (
          <State title="Not available" copy="Fees are not enabled for this school." />
        ) : null}
        {error ? <State title="Could not refresh" copy={error} /> : null}
        {!loading && enabled && !error && invoices.length === 0 ? (
          <State title="No invoices" copy="New school charges will appear here." />
        ) : null}
        {!loading
          ? invoices.map((invoice) => {
              const structure = structures[invoice.fee_structure_id];
              const currency = structure?.currency ?? "GHS";
              const payment = payments[invoice.id];
              return (
                <View key={invoice.id} style={styles.card}>
                  <View style={styles.row}>
                    <View style={styles.details}>
                      <Text style={styles.name}>{structure?.name ?? "School fee"}</Text>
                      <Text style={styles.meta}>
                        {students[invoice.student_id] ?? "Linked learner"} · Due{" "}
                        {invoice.due_date ?? "date pending"}
                      </Text>
                    </View>
                    <Text style={styles.status}>{payment?.status ?? invoice.status}</Text>
                  </View>
                  <View style={styles.amountRow}>
                    <View>
                      <Text style={styles.label}>Amount</Text>
                      <Text style={styles.amount}>
                        {formatMoney(invoice.amount_cents, currency)}
                      </Text>
                    </View>
                    <View>
                      <Text style={styles.label}>Balance</Text>
                      <Text style={[styles.amount, { color: theme.brand }]}>
                        {formatMoney(invoice.balance_cents, currency)}
                      </Text>
                    </View>
                  </View>
                  {features.has("online_payments") && payment?.status === "pending" ? (
                    <PrimaryButton
                      label={initiating === payment.id ? "Starting checkout…" : "Pay securely"}
                      disabled={initiating !== ""}
                      onPress={() => void initiate(payment)}
                    />
                  ) : null}
                </View>
              );
            })
          : null}
      </ScrollView>
    </Screen>
  );
}

function formatMoney(cents: number, currency: string) {
  try {
    return new Intl.NumberFormat(undefined, { style: "currency", currency }).format(cents / 100);
  } catch {
    return `${currency} ${(cents / 100).toFixed(2)}`;
  }
}
function State({ title, copy }: { title: string; copy: string }) {
  return (
    <View accessibilityLiveRegion="polite" style={styles.state}>
      <Text style={styles.stateTitle}>{title}</Text>
      <Text style={styles.meta}>{copy}</Text>
    </View>
  );
}
const styles = StyleSheet.create({
  content: { gap: 13, paddingBottom: 36 },
  title: { color: colors.ink, fontSize: 30, fontWeight: "900" },
  intro: { color: colors.muted, lineHeight: 21, marginBottom: 5 },
  card: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    padding: 17,
    gap: 16,
  },
  row: { flexDirection: "row", alignItems: "center", gap: 12 },
  details: { flex: 1, gap: 5 },
  name: { color: colors.ink, fontSize: 16, fontWeight: "900" },
  meta: { color: colors.muted, lineHeight: 20 },
  status: { color: colors.ink, fontSize: 12, fontWeight: "900", textTransform: "uppercase" },
  amountRow: { flexDirection: "row", justifyContent: "space-between", gap: 24 },
  label: { color: colors.muted, fontSize: 12, fontWeight: "700", marginBottom: 4 },
  amount: { color: colors.ink, fontSize: 18, fontWeight: "900" },
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
});
