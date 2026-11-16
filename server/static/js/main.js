// Expenser's own behaviour. The shared theme's theme.js (loaded first) does
// the rest: light/dark, the navigation highlight, the side sheet (#sheet:
// themeShowSheet, themeHideSheet), toasts (#toast: themeShowToast,
// themeHideToast) and the settings menu.

// --- Grocery forms: total = quantity x unit price ---
function setupGroceryCalc(form) {
  const qty = form.querySelector('[name="quantity"]');
  const unit = form.querySelector('[name="unit_price"]');
  const total = form.querySelector('[name="total_price"]');

  if (!qty || !unit || !total) {
    return;
  }

  const calc = () => {
    const q = parseFloat(qty.value) || 0;
    const u = parseFloat(unit.value) || 0;
    total.value = (q * u).toFixed(2);
  };

  qty.addEventListener("input", calc);
  unit.addEventListener("input", calc);
}

const actionDialog = document.getElementById("sheet");
if (actionDialog) {
  actionDialog.addEventListener("htmx:afterSettle", (event) => {
    const form = event.target.querySelector(".new-expense-form");
    if (form) {
      setupGroceryCalc(form);
    }
  });
}

// A required field folded inside a closed <details> can't be focused by the
// browser's validation, so open it when one of its fields is invalid.
document.addEventListener(
  "invalid",
  (event) => {
    const details = event.target.closest("details");
    if (details && !details.open) {
      details.open = true;
    }
  },
  true,
);

// --- Keyboard shortcuts (desktop): N adds an expense, Escape closes the panel ---
document.addEventListener("keydown", (event) => {
  if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey) {
    return;
  }
  const actionDialog = document.getElementById("sheet");
  const dialogOpen = actionDialog && actionDialog.open;

  if (event.key === "Escape" && dialogOpen) {
    themeHideSheet();
    return;
  }

  const target = event.target;
  const typing =
    target.closest("input, textarea, select, [contenteditable]") !== null;
  if ((event.key === "n" || event.key === "N") && !typing && !dialogOpen) {
    const addButton = document.querySelector(".month-toolbar .add-btn");
    if (addButton && addButton.offsetParent !== null) {
      event.preventDefault();
      addButton.click();
    }
  }
});

// --- List | Chart switcher follows whatever is loaded into the section ---
// Anything can load the chart or the list (the tabs, the mini chart, the
// month arrows), so the highlight is set from the request that filled the
// section rather than from which button was clicked.
function sectionViewOf(path) {
  return (path || "").split("?")[0].endsWith("/chart") ? "chart" : "list";
}

function syncSectionButtons(path) {
  const view = sectionViewOf(path);
  document.querySelectorAll(".section-button").forEach((button) => {
    const buttonView = sectionViewOf(button.getAttribute("hx-get"));
    button.classList.toggle("active", buttonView === view);
  });
}

document.body.addEventListener("htmx:afterSwap", (event) => {
  const target = event.detail.target;
  if (target && (target.id === "section-content" || target.id === "month-view")) {
    syncSectionButtons(event.detail.pathInfo.requestPath);
  }
});

// A full page load of /<section>/chart renders the list button as active;
// fix the highlight from the address bar.
syncSectionButtons(location.pathname);
document.body.addEventListener("htmx:historyRestore", () =>
  syncSectionButtons(location.pathname),
);
