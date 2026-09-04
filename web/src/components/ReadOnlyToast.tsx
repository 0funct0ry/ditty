/** SPEC.md §10.1 read-only: a one-shot inline toast on a keystroke, never a
 * modal. */
export function ReadOnlyToast() {
  return (
    <div className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-md border border-line bg-ink-soft px-3 py-2 font-sans text-sm text-[#E4E7EC] motion-reduce:transition-none">
      This session is read-only
    </div>
  );
}
