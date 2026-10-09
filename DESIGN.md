# Design — Project Identity

> This document is project-long-lived. Tokens are not changed without
> the Architect's approval. Developers MUST use these tokens
> instead of improvising their own colors/spacings.

## Style Direction

A calm, professional "workshop console": light grey work surfaces, deep slate anthracite text, one dependable blue accent for every action, and clearly separated semantic status colours for the order state machine — restrained like Linear/Stripe, no decoration, no external resources (AC-32).

## Colors

- `--color-bg`: **#F6F7F9**
- `--color-surface`: **#FFFFFF**
- `--color-surface_sunken`: **#EEF1F4**
- `--color-fg`: **#1F2933**
- `--color-fg_strong`: **#111820**
- `--color-muted`: **#6B7280**
- `--color-border`: **#DDE1E6**
- `--color-border_strong`: **#C3CAD3**
- `--color-accent`: **#2563EB**
- `--color-accent_hover`: **#1D4ED8**
- `--color-accent_active`: **#1E40AF**
- `--color-accent_subtle`: **#EFF4FF**
- `--color-focus_ring`: **#93B4FB**
- `--color-danger`: **#B91C1C**
- `--color-danger_subtle`: **#FEF2F2**
- `--color-success`: **#047857**
- `--color-success_subtle`: **#EAF7F1**
- `--color-warning`: **#B45309**
- `--color-warning_subtle`: **#FEF4E6**
- `--color-status_angefragt`: **#5B6472**
- `--color-status_angefragt_bg`: **#F1F3F5**
- `--color-status_bestaetigt`: **#1D4ED8**
- `--color-status_bestaetigt_bg`: **#EFF4FF**
- `--color-status_in_arbeit`: **#B45309**
- `--color-status_in_arbeit_bg`: **#FEF4E6**
- `--color-status_fertig`: **#047857**
- `--color-status_fertig_bg`: **#EAF7F1**
- `--color-status_abgeholt`: **#374151**
- `--color-status_abgeholt_bg`: **#ECEEF1**
- `--color-overlay`: **rgba(17,24,32,0.45)**

## Typography

- `font_family`: system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif
- `font_mono`: ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace
- `heading_weight`: 600
- `body_weight`: 400
- `size_xs`: 12px
- `size_sm`: 14px
- `size_md`: 16px
- `size_lg`: 20px
- `size_xl`: 24px
- `size_2xl`: 32px
- `line_body`: 1.5
- `line_heading`: 1.25
- `tracking_heading`: -0.01em
- `numeric`: font-variant-numeric: tabular-nums for all money, hours, kilometres and counters

## Spacing Scale

- `--space-0`: 4px
- `--space-1`: 8px
- `--space-2`: 12px
- `--space-3`: 16px
- `--space-4`: 24px
- `--space-5`: 32px
- `--space-6`: 48px

## Border-Radii

- `--radius-sm`: 4px
- `--radius-md`: 8px
- `--radius-lg`: 12px
- `--radius-pill`: 999px

## Components

### Button

Variants: primary (bg=accent, fg=#FFFFFF), secondary (bg=surface, fg=fg, 1px border=border_strong), ghost (transparent, fg=accent), danger (bg=danger, fg=#FFFFFF). Size md: min-height 44px, padding 12px/24px, radius md, font-size 16px/weight 600. Size sm: min-height 44px on touch layouts, 36px only on desktop tables, padding 8px/16px, font-size 14px — never below 44x44px for the primary tap target. States: default; hover = bg accent_hover (primary) / surface_sunken (secondary); active = bg accent_active, transform translateY(1px); focus-visible = 2px outline focus_ring with 2px offset, always visible for keyboard; disabled = opacity 0.5, cursor not-allowed, no hover change, aria-disabled=true (used visibly for the 'not yet available' status transitions of AC-23/AC-20). Every visible button either performs its function or is visibly disabled — no dead controls. Loading: label replaced by 'Wird gesendet …' plus inline 16px spinner, button stays disabled.

### Input / Select / Textarea

min-height 44px, radius md, 1px border=border_strong, bg=surface, padding 12px/12px, font-size 16px (prevents iOS zoom), placeholder fg=muted. Focus: 1px accent border + 3px focus_ring shadow (no layout shift). Error state: border=danger, 3px danger_subtle shadow, aria-invalid=true. Disabled: bg=surface_sunken, fg=muted. Textarea: min-height 96px, resize vertical. Error colour is applied ONLY after the field was touched or the form was submitted (AC-13: untouched form shows no error and no red border).

### FormField

Label above input, font-size 14px/weight 600, margin-bottom 8px, required fields marked with ' *' and a legend. Helper text 12px muted below. Error text 12px danger below the input with a 12px warning glyph, announced via aria-describedby and role=alert. Vertical rhythm: 16px between fields, 24px before the submit row.

### Card

bg=surface, 1px border=border, radius lg (12px), padding 24px (16px below 640px), shadow 0 1px 2px rgba(17,24,32,0.06). Card header: 14px/600 muted uppercase-ish label + optional right-aligned action; header to body spacing 16px. Cards never nest more than one level.

### TopNav

Sticky, height 56px (mobile) / 64px (desktop), bg=surface, bottom border 1px border. Left: wordmark 'Werkstatt-Portal' 16px/600 fg_strong. Right: area switch 'Kundenbereich' | 'Werkstattbereich' (accent underline for active, 44px tap height) and, when logged in, the employee e-mail (muted, truncated) plus a 'Abmelden' ghost button. Mobile: switches collapse to a 44px icon button opening a full-width sheet.

### PageHeader

Title 24px (32px on desktop) weight 600 fg_strong, optional one-line subtitle 16px muted, 24px gap before content. Optionally a primary action right-aligned (wraps below the title under 640px).

### StatusBadge

Pill radius, padding 4px/12px, font-size 12px/600, uppercase tracking 0.02em, 1px border in the same hue, includes a 6px dot. Labels and colours are fixed and identical everywhere the status appears: 'angefragt' fg=status_angefragt bg=status_angefragt_bg, 'bestätigt' fg=status_bestaetigt bg=status_bestaetigt_bg, 'in Arbeit' fg=status_in_arbeit bg=status_in_arbeit_bg, 'fertig' fg=status_fertig bg=status_fertig_bg, 'abgeholt' fg=status_abgeholt bg=status_abgeholt_bg. Never colour-only: the label text always carries the meaning.

### StatusTimeline

Vertical list of the status log (AC-05/AC-11): 12px rail with 10px dot per entry (dot colour = target status colour), to the right timestamp DD.MM.YYYY, HH:MM (UTC) in mono 14px and the target status as StatusBadge. 16px between entries, newest entry first, muted-lighter for older entries.

### DataTable (order list)

Desktop: header row 12px/600 muted, sticky, bg=surface_sunken; body rows 48px min-height, 1px bottom border=border, hover bg=accent_subtle; columns: Auftragsnummer (mono), Kennzeichen (mono), Fahrzeug, Wunschtermin, Status (StatusBadge), Aktion (right-aligned, 44px). Mobile (<900px): rows become stacked cards — label/value pairs 12px muted left, value 16px right, order number 14px mono on top, action button full width. Numbers right-aligned, tabular-nums. Sortable header affordance is a 16px chevron; empty result shows EmptyState, never a blank table.

### FilterBar / SearchInput

Row above the table, 12px gaps, wraps below 640px. Status filter: segmented control of the five statuses plus 'Alle', each segment min-height 44px, radius md, active = accent_subtle bg + accent text + accent border. Licence search: Input with 16px magnifier icon, prefix-free, case-insensitive, trims and uppercases the input (mono font), 300ms debounce, clear '×' 44px button when filled. Filter state is reflected in the URL query so a reload keeps it.

### FormLayout (Terminanfrage)

Single column, max-width 720px, 16px between fields, grouped in sections 'Fahrzeug' / 'Termin' / 'Problem' separated by 32px and a 1px divider. Position rows (Arbeitszeit/Teil) start with one empty row: Beschreibung (flex), Menge/Stunden (100px, right-aligned, numeric), Einzelpreis bzw. Stundensatz (120px, right-aligned), line total (120px, mono, read-only), 'Entfernen' ghost 44px. 'Position hinzufügen' as secondary button. Summary block right-aligned: Netto, 19 % MwSt., Brutto — Brutto 20px/600. Submit row: primary 'Termin anfragen' left, muted note about the next step right.

### StatusLookup

Centred card, max-width 560px: heading 'Status abfragen', two fields 'Auftragsnummer' and 'Kennzeichen' side by side (stacked below 640px), primary button full width on mobile. On mismatch a single Alert with the message 'Zu diesen Angaben wurde kein Auftrag gefunden.' — never a partial result, never a hint which of the two values is wrong (AC-11). On success the result card below shows StatusBadge 20px, Auftragsnummer mono, Fahrzeug and the StatusTimeline.

### InvoiceView

Two columns ≥900px (positions left, summary right, summary sticky top 80px), single column below. Positions table: Beschreibung, Menge/Stunden, Einzelpreis, Betrag — all money right-aligned, mono, tabular-nums. Summary card: Netto, 'MwSt. 19 %', divider, 'Brutto' 20px/600 with 1px accent top border. Header shows Rechnungsnummer mono, Auftragsnummer mono, Rechnungsdatum DD.MM.YYYY and 'Alle Beträge in Euro, gerundet auf Cent.' Footnote: 'Es wird keine E-Mail versendet; die Rechnung liegt im Postausgang.'

### StatCard (dashboard)

Grid of 3 cards (stacked below 640px): label 12px/600 muted uppercase, value 32px/600 fg_strong tabular-nums, one-line context 12px muted. Units always in the value column: '7', '4', '12.480,00 €'. Optional 24px accent-tinted icon square, radius md.

### Alert

Inline block, radius md, padding 12px/16px, 1px border + subtle bg per tone: error (danger/danger_subtle), success (success/success_subtle), warning (warning/warning_subtle), info (accent/accent_subtle). 16px glyph, title 14px/600, body 14px, role=alert for errors. Messages are always the German, human-readable text derived from the API error body (code + message) — a technical error page is never shown (AC-22).

### Toast

Bottom-centre on mobile, bottom-right on desktop, max-width 360px, bg=fg_strong, fg=#FFFFFF, radius md, padding 12px/16px, 8px offset from the viewport edge, auto-dismiss after 5s, dismiss '×' 44px. Used for confirmations such as 'Status auf „bestätigt' gesetzt.' Never used for validation errors — those stay inline.

### Modal / ConfirmDialog

Centred, max-width 480px, bg=surface, radius lg, padding 24px, overlay=overlay, 2px focus_ring for keyboard focus trapped inside. Title 20px/600, body 16px, actions right-aligned ('Abbrechen' secondary, confirm primary/danger). Required for irreversible steps ('Auftrag abholen bestätigen?'), not for navigation.

### EmptyState

Centred, 48px vertical padding, 32px muted glyph, title 16px/600, one-line hint 14px muted, optional secondary action. Copy is state-specific: 'Noch keine Aufträge in diesem Status.', 'Für diesen Auftrag liegt noch keine Rechnung vor.', 'Keine Positionen erfasst.'

### LoginCard

Centred card max-width 420px with 'E-Mail' and 'Passwort' fields, primary button 'Anmelden' full width, 44px. On 401 exactly one generic Alert: 'E-Mail oder Passwort ist falsch.' — never revealing which part was wrong (AC-15). During submit the button shows the loading state; the note 'Zugang für Werkstattmitarbeiter' sits muted beneath.

### LegalFooter

Present on every page (AC-31): 1px top border, padding 24px, 12px muted, links 'Impressum' and 'Datenschutz' as visible 14px text links with 44px tap height, min contrast 4.5:1, no third-party embeds.

### Skeleton / Spinner

Skeleton: bg=surface_sunken, radius sm, 1px fade pulse, one line per expected text row, matching final heights so nothing jumps. Spinner: 16px/20px, 2px stroke, accent colour, with a visually hidden 'Wird geladen …'.

## Layout Principles

- Container: max-width 1200px, centred, horizontal padding 16px (mobile) / 24px (≥640px) / 32px (≥1200px). Reading and form flows use a narrow container of max-width 720px; single-decision flows (Status lookup, login) max-width 560px.
- 8px base grid: every margin, padding and gap is a token value (4/8/12/16/24/32/48px). 8px inside controls, 16px between fields and rows, 24px between cards, 32-48px between page sections. No arbitrary pixel values.
- Breakpoints: 640px (mobile → tablet: form rows stack to one column), 900px (tablet → desktop: tables appear, order list switches from cards to table, two-column layouts), 1200px (content caps, side paddings grow). Mobile first; every screen is fully operable at 360px width with no horizontal scroll and no sideways-scrolling table.
- Responsiveness rule (AC-23): all layouts use flex/grid with wrap; primary actions stack full width below 640px; tap targets ≥44x44px everywhere; the sticky top navigation never covers focus or content.
- Spacing between a page header and its content is 24px; between two cards 16px; card padding 24px (16px under 640px); the vertical rhythm inside a card is 16px.
- FOCUS is always visible: 2px focus_ring outline with 2px offset on all interactive elements; colour is never the only carrier of meaning (status badges, errors and success states also carry text or a glyph); body text contrast ≥4.5:1, muted text ≥4.5:1 on bg.
- VALUE FORMATTING — one single form all over the product, customer and workshop area alike, so parallel-built screens format a value identically: date 'DD.MM.YYYY' (07.03.2025), time 'HH:MM' 24-hour, timestamp 'DD.MM.YYYY, HH:MM (UTC)' with the literal suffix '(UTC)' and no local timezone conversion anywhere, duration/hours '2,5 h' (hours with at most one decimal, comma as decimal separator, always with the unit), money from integer cents '1.234,56 €' (thousands separator '.', decimal separator ',', exactly two decimals, thin no-break space before '€'), percentage '19 %', mileage '123.456 km', order number 'AW-2025-000123' and licence plate 'M-AB 1234' both uppercase in the mono font, status labels verbatim 'angefragt', 'bestätigt', 'in Arbeit', 'fertig', 'abgeholt'. All numbers use tabular-nums and are right-aligned in tables and summaries.
- Form conventions: labels always above the field, required fields marked with ' *', helpers under the field, errors under the field in danger colour — but only after touch or submit; the submit button sits at the start of the submit row, and it is disabled only while a request is running, never as a substitute for validation.
- Table conventions: one record per row, order number and licence plate in mono on the left, status and action on the right; row actions are 44px; destructive steps require the ConfirmDialog; the same empty, loading and error states apply in every table and list.
- Accessibility baseline: semantic landmarks (header/nav/main/footer), one h1 per page, a logical heading order, aria-live for toasts and the timeline, keyboard operability of tabs, filters, modals and tables exactly as with the mouse, no information conveyed by animation alone.
- Privacy by design in the UI (AC-32/AC-33): fonts are system fonts shipped with the OS, no web fonts, no CDN, no analytics, no third-party iframes; every asset comes from the app's own build. Never render personal data (name, e-mail, phone, licence plate) into log output, URL query strings, page titles or browser storage.
