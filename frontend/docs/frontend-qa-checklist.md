# Frontend QA Checklist

Use this checklist to validate critical flows before shipping changes.

## Authentication
- [ ] New user can open the app and see the sign-in form.
- [ ] Existing account can sign in and sign out (verify both success and error states).
- [ ] Theme toggle persists preference across reloads.

## Dashboard
- [ ] Left sidebar toggles open/closed on desktop and mobile widths.
- [ ] Memory feed loads, sorts newest first, and the “Load more memories” button fetches the next page.
- [ ] Creating a memory via the compact form refreshes the feed and category sidebar.
- [ ] Document mode saves correctly and returns to the compact form.
- [ ] Editing and deleting a memory update the feed without errors.
- [ ] Loading and error banners render when API calls fail (temporarily mock failures).

## Categories
- [ ] Category list renders, including empty state and note counts.
- [ ] Expanding a category shows preview memories, handling loading/empty cases.
- [ ] Recent memories section links to individual memories.

## Graph Visualisation
- [ ] Graph loads without console errors and selects nodes from the feed/sidebar.
- [ ] “View in graph” focuses the correct node.
- [ ] Graph errors surface via the global error boundary.

## Accessibility
- [ ] Skip link focuses the main content.
- [ ] All interactive controls are keyboard accessible with visible focus styles.
- [ ] Announcements (notifications, status messages) read correctly via screen readers.

## Tooling
- [ ] `npm run lint`, `npm run format`, `npm run typecheck`, and `npm test` succeed locally.

Capture screenshots or screen recordings for significant UI changes and attach to the release notes.
