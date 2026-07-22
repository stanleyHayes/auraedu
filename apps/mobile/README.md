# AuraEDU Mobile

Expo/React Native shell for the teacher, parent, and student portals (EP-48, L7).

## Local development

```bash
pnpm install
cd apps/mobile
cp .env.example .env.local
pnpm ios      # or pnpm android, pnpm start
```

Development may use HTTP for a local gateway. Preview and production configuration rejects
non-HTTPS API endpoints, embedded credentials, query parameters and fragments.

## Release readiness

The public version starts at `1.0.0`; EAS owns iOS build numbers and Android version codes and
increments them for production builds. Preview and production builds also require the linked Expo
project ID so push registration cannot disappear silently.

The native configuration includes an app-level Apple required-reason privacy manifest and blocks
unused Android location, camera, contacts, media, microphone and advertising-ID permissions. The
release check verifies those declarations plus the exact icon dimensions before an authenticated
build can begin.

After an organisation owner has linked the project with `eas init`, configure the EAS `preview`
and `production` environments with `EAS_PROJECT_ID` and `EXPO_PUBLIC_API_URL`. Before building:

```bash
APP_ENV=production \
EXPO_PUBLIC_API_URL=https://api.auraedu.com \
EAS_PROJECT_ID=<linked-project-uuid> \
pnpm release:check

eas build --profile production --platform all
eas submit --profile production --platform ios
eas submit --profile production --platform android
```

The App Store Connect application/team and Google Play service account remain account-owned EAS
credentials; they do not belong in this repository. `eas.json` intentionally contains no fake IDs
or credential paths.

## OTA updates

Preview and production binaries use separate EAS Update channels and the app-version runtime
policy. Publish JavaScript-only fixes to the matching environment; never test a release candidate
on the production channel:

```bash
eas update --channel preview --environment preview --message "Describe the candidate"
eas update --channel production --environment production --message "Describe the release"
```

Any native dependency, Expo configuration, permission, icon, or runtime-version change requires a
new store binary. OTA updates are only for JavaScript and bundled asset changes compatible with the
already shipped `runtimeVersion`.

## Scope

This shell is tenant-aware and role-aware. Admin features are intentionally out of
scope; the mobile app is for school community users only.

At startup the app resolves the active tenant, applies its validated logo and
brand colour, loads a fail-closed feature snapshot, and creates the shared typed
Gateway client. The Notices tab reads live, server-scoped announcements for the
signed-in teacher, parent, or student audience. It explains push delivery before asking for OS
permission; sign-in never triggers a permission prompt. If alerts are denied, notices remain
available in-app and the user can open device settings from the same screen.

Session state is schema-validated after secure restoration. Access tokens refresh through the
Identity rotation endpoint before expiry and once after an unexpected `401`; concurrent requests
share one rotation. A failed or identity-mismatched refresh clears the local session. Sign-out
removes the mobile installation and revokes the server refresh session before deleting local keys.

Parent attendance and results are restricted to linked learners. Teacher class,
roster, attendance, and score APIs are restricted server-side through the private
identity → staff → assigned class → active roster scope chain. The teacher mobile
attendance register consumes that boundary and supports safe daily bulk marking.
Teacher score entry uses the same scope for assigned-class assessment discovery and
for create/update authorization on each learner score.
Student assignments are restricted to published work targeting the learner's current
class; draft and cross-class assignments are hidden server-side.

Every native page introduction exposes the same contextual walkthrough used by its visible help
sheet and optional British-English narration. The four-tab first-login coach tour is persisted by
role, tenant and user, follows the operating-system reduced-motion preference and can be replayed
from Profile without clearing its completion record.

Authenticated report-card PDFs use the operating-system cache only while the user opens or shares
them. The app deletes the local file in a `finally` path after both successful and failed flows.
