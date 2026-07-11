"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

export async function logoutAction() {
  const jar = await cookies();
  jar.delete("auraedu_access_token");
  jar.delete("auraedu_user");
  redirect("/login");
}
