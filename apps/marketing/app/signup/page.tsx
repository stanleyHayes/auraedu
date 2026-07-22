import { Suspense } from "react";
import { SignupForm } from "./signup-form";
import { Skeleton } from "@auraedu/ui";

export const metadata = {
  title: "Start your school",
  description: "Submit a secure AuraEDU onboarding request for your school.",
};

export default function SignupPage() {
  return (
    <Suspense
      fallback={
        <div className="mx-auto max-w-2xl px-6 py-16">
          <Skeleton className="mx-auto h-8 w-48" />
          <Skeleton className="mx-auto mt-3 h-4 w-72" />
          <div className="mt-10 space-y-5 rounded-lg border border-border bg-surface p-6 sm:p-8">
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
          </div>
        </div>
      }
    >
      <SignupForm />
    </Suspense>
  );
}
