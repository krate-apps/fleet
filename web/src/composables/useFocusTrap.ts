import { nextTick, onBeforeUnmount, watch, type Ref } from "vue";

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

function listFocusable(root: HTMLElement): HTMLElement[] {
  return [...root.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => {
    if (el.hasAttribute("disabled") || el.getAttribute("aria-hidden") === "true") return false;
    if (el.tabIndex < 0) return false;
    // offsetParent is null for fixed/positioned-out nodes; still allow visible ones.
    const style = window.getComputedStyle(el);
    return style.visibility !== "hidden" && style.display !== "none";
  });
}

/**
 * PD-UX-20: trap Tab inside `container` while `active`, restore prior focus on exit.
 */
export function useFocusTrap(container: Ref<HTMLElement | null | undefined>, active: Ref<boolean>) {
  let previouslyFocused: HTMLElement | null = null;

  function onKeydown(ev: KeyboardEvent) {
    if (ev.key !== "Tab") return;
    const root = container.value;
    if (!root) return;
    const nodes = listFocusable(root);
    if (!nodes.length) {
      ev.preventDefault();
      return;
    }
    const first = nodes[0];
    const last = nodes[nodes.length - 1];
    const current = document.activeElement as HTMLElement | null;
    if (ev.shiftKey) {
      if (current === first || !root.contains(current)) {
        ev.preventDefault();
        last.focus();
      }
    } else if (current === last || !root.contains(current)) {
      ev.preventDefault();
      first.focus();
    }
  }

  async function activate() {
    previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    document.addEventListener("keydown", onKeydown, true);
    await nextTick();
    const root = container.value;
    if (!root) return;
    const nodes = listFocusable(root);
    const preferred =
      root.querySelector<HTMLElement>("[data-autofocus]") ??
      nodes.find((el) => el.tagName !== "BUTTON" || !el.classList.contains("fleet-icon-btn")) ??
      nodes[0];
    preferred?.focus();
  }

  function deactivate() {
    document.removeEventListener("keydown", onKeydown, true);
    if (previouslyFocused && document.contains(previouslyFocused)) {
      previouslyFocused.focus();
    }
    previouslyFocused = null;
  }

  watch(
    active,
    (on) => {
      if (on) void activate();
      else deactivate();
    },
    { immediate: true },
  );

  onBeforeUnmount(() => {
    document.removeEventListener("keydown", onKeydown, true);
  });
}
