import Image from "next/image";
import { cn } from "@auraedu/ui";

interface AuraEduLogoProps {
  className?: string;
  priority?: boolean;
  tone?: "dark" | "light";
  variant?: "lockup" | "mark";
}

/** The platform brand lockup. School/tenant identities remain separate. */
export function AuraEduLogo({
  className,
  priority = false,
  tone = "dark",
  variant = "lockup",
}: AuraEduLogoProps) {
  const mark = variant === "mark";
  const src = mark ? "/brand/auraedu-mark.svg" : `/brand/auraedu-logo-${tone}.svg`;

  return (
    <Image
      src={src}
      alt="AuraEDU"
      width={mark ? 48 : 208}
      height={48}
      priority={priority}
      className={cn(mark ? "size-9" : "h-9 w-auto", className)}
    />
  );
}
