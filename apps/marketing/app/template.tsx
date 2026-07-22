import { PageTransition } from "@/components/motion-primitives";

export default function MarketingTemplate({ children }: { children: React.ReactNode }) {
  return <PageTransition>{children}</PageTransition>;
}
