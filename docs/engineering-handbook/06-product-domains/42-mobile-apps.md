# Chapter 42: Mobile Apps

## Purpose

AuraEDU Mobile gives teachers, parents and students a trustworthy, school-branded companion for the work that benefits from being available on a personal device. It must feel like the same product as the web portals while respecting the tighter privacy, interruption and connectivity boundaries of mobile operating systems.

---

## Scope

- One Expo application for `teacher`, `parent` and `student` roles.
- Authentication, tenant branding, announcements, attendance, assessments, assignments, results, fees, report cards, recommendations and career guidance.
- Push registration, secure local session storage, EAS Build, EAS Submit and EAS Update.
- School administration and platform administration remain web-only.

---

## Principles

- Mobile is a focused companion, not a compressed administration console.
- A permission prompt follows a clear user action and explanation; sign-in alone is never consent.
- Sensitive learner documents exist on-device only for the shortest practical time.
- The server remains authoritative for identity, tenant, role, feature and learner scope.
- Native configuration changes require a new binary. OTA updates never cross runtime versions.
- Marketing, portal and native surfaces share the same design tokens, information hierarchy and reduced-motion behavior.

---

## Business Rules

- Only teachers, parents and students may establish a mobile session.
- Parents see learners linked through the authoritative guardian relationship.
- Teachers act only on assigned classes and active rosters.
- Students see only work and records published to their active enrolment.
- AI recommendations and guidance remain advisory and respect human-review visibility rules.
- Push notifications are optional. Denial cannot prevent access to the in-app Notices workspace.
- AuraEDU does not use advertising identifiers, tracking domains or cross-app tracking.

---

## Technical Rules

- Access and refresh tokens are schema-validated and stored only with `expo-secure-store`.
- Tenant preference may use Async Storage; credentials, tokens and learner records may not.
- Every Gateway request carries the active access token and canonical tenant code.
- Session refresh is single-flight, verifies identity continuity and clears invalid state.
- Production and preview builds require HTTPS API origins without credentials, query strings or fragments.
- Push tokens are registered only after an explicit in-context user action or when permission was already granted.
- Downloaded report-card PDFs use the cache directory and are deleted in a `finally` path after success or failure.
- Android blocks unused location, camera, contacts, media, microphone and advertising-ID permissions.
- The app-level Apple privacy manifest aggregates the reviewed required-reason API declarations used by React Native, Expo and Async Storage dependencies.
- Expo SDK 57 targets Android API 36. Recheck the SDK/platform matrix before every release train.
- Production, preview and development use separate EAS environments, update channels and credentials.
- Every page guide uses one content source for visible numbered steps and `expo-speech` narration; speech stops when the sheet closes or the screen unmounts.
- The first-login coach tour is scoped by role, tenant and user in Async Storage, supports explicit replay and follows the operating-system reduced-motion preference.

---

## Architecture

```text
Expo Router UI
    |
    +-- AuthProvider
    |     +-- SecureStore session and device identifier
    |     +-- tenant branding and live feature snapshot
    |     +-- single-flight token refresh
    |     `-- user-initiated push registration
    |
    +-- typed Gateway client
    |     `-- API Gateway -> domain services
    |
    `-- native capabilities
          +-- Notifications
          +-- page-guide speech and coach marks
          +-- temporary FileSystem cache
          `-- OS share sheet
```

The app never reads a service database or trusts a locally selected learner. Parent, teacher and student scope is resolved at the corresponding server boundary.

---

## Best Practices

- Explain the benefit before invoking an operating-system permission prompt.
- Render a useful in-app alternative when a native permission is denied.
- Test compact phones, large phones and iPad rendering even when the product is phone-first.
- Use semantic headings, button roles, accessible labels and live regions.
- Respect the operating-system reduced-motion preference in every shared animation primitive.
- Keep store metadata, privacy answers and screenshots aligned with actual runtime behavior.
- Run `pnpm release:check`, both platform exports and the signed preview build before production submission.

---

## Examples

- The Notices screen explains push alerts and asks only after the user presses **Enable push alerts**. A denied permission routes the user to device settings while in-app notices remain available.
- A parent opens a report card through the OS share sheet. AuraEDU removes the authenticated PDF from its cache as soon as that flow returns.
- A teacher opens **My classes** to see only server-scoped assignments, then enters the daily register. The Reports workspace shows draft state honestly and exposes a PDF action only after publication.
- A first-time user receives a four-step coach tour of Today, My work, Notices and Profile. Profile can replay it, while each page's help button opens the same steps used by speech narration.
- A production update that changes only JavaScript and bundled assets may use the production EAS channel when its runtime version matches the installed binary.

---

## Anti-patterns

- Requesting notification permission immediately after sign-in.
- Storing refresh tokens in Async Storage.
- Leaving downloaded learner documents in a stable cache path.
- Embedding a learner ID chosen by the client without server-side ownership validation.
- Adding an SDK that introduces a sensor, advertising or tracking permission without product, privacy and security review.
- Shipping an OTA update after changing native dependencies, permissions, icons or runtime configuration.
- Claiming store readiness from a Metro export alone; signed binaries and store acceptance are separate evidence.

---

## Checklist

- [ ] Typecheck, lint and mobile tests pass.
- [ ] `APP_ENV=production`, the HTTPS Gateway origin and linked EAS project pass `release:check`.
- [ ] iOS and Android production bundles export successfully.
- [ ] App icon and adaptive icon are valid 1024x1024 PNG files.
- [ ] Required-reason API declarations match the locked native dependencies.
- [ ] Unused Android sensitive permissions remain blocked.
- [ ] Push consent, denial, Settings recovery and token removal are tested on devices.
- [ ] Small-phone, large-phone and iPad layouts are visually reviewed with reduced motion on and off.
- [ ] Page-guide sheets, narration stop behavior and the role/tenant/user-scoped coach tour are verified with screen-reader and reduced-motion settings.
- [ ] Signed preview builds pass authentication, tenant isolation and core-role smoke tests.
- [ ] App Store Connect and Google Play privacy disclosures match the data-flow inventory.
- [ ] Production-channel OTA promotion is verified on a signed preview binary.

---

## Definition of Done

- Automated configuration, security, accessibility, type, lint and bundle checks pass.
- An organisation-owned Expo project is linked and production credentials are valid.
- Signed iOS and Android builds pass the role and tenant-isolation smoke matrix on real devices.
- App Store Connect and Google Play accept the binaries and required privacy metadata.
- A preview OTA update is verified before promotion to the production channel.
- Sanitized build, submission and OTA evidence is retained in the release evidence manifest.

---

## References

- [Mobile implementation](../../../apps/mobile/README.md)
- [Release evidence manifest](../../../release/evidence/manifest.json)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Expo app-store guidance](https://docs.expo.dev/distribution/app-stores/)
- [Expo privacy manifests](https://docs.expo.dev/guides/apple-privacy/)
- [Apple privacy manifests](https://developer.apple.com/documentation/bundleresources/privacy-manifest-files)
- [Google Play target API requirements](https://support.google.com/googleplay/android-developer/answer/11926878)
