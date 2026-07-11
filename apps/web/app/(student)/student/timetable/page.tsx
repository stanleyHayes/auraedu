import { CalendarDays } from "lucide-react";
import { EmptyState } from "@auraedu/ui";

export default function StudentTimetablePage() {
  return (
    <EmptyState
      icon={<CalendarDays className="size-8" />}
      title="Timetable"
      description="Your class timetable will appear here once it is published."
    />
  );
}
