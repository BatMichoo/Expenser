document.addEventListener("DOMContentLoaded", () => {
  const themeToggle = document.getElementById("theme-toggle");
  const body = document.body;

  function setTheme(theme) {
    if (theme === "dark") {
      body.classList.add("dark");
      localStorage.setItem("theme", "dark");
    } else {
      body.classList.remove("dark");
      localStorage.setItem("theme", "light");
    }
  }

  // Initialize Theme
  const savedTheme = localStorage.getItem("theme");
  if (savedTheme) {
    setTheme(savedTheme);
  } else if (
    window.matchMedia &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  ) {
    setTheme("dark");
  } else {
    setTheme("light");
  }

  // Theme Toggle Listener
  if (themeToggle) {
    themeToggle.addEventListener("click", () => {
      if (body.classList.contains("dark")) {
        setTheme("light");
      } else {
        setTheme("dark");
      }

      // const chart = document.getElementById("chart");
      // if (chart) {
      //   chart.
      //
      // }
    });
  }

  // Navigation Logic
  const trackerNavButtons = document.querySelectorAll(
    ".tracker-nav-button:not(.section-button)",
  );

  const setActiveTrackerNavButton = (path) => {
    // Get the prefix from the base tag, e.g., "/expenser"
    const baseHref =
      document.querySelector("base")?.getAttribute("href") || "/";
    const prefix = baseHref.replace(/\/$/, "");

    // Remove the prefix from the current path to get the "inner" path
    // e.g., "/expenser/settings" -> "/settings"
    let internalPath = path;
    if (prefix && internalPath.startsWith(prefix)) {
      internalPath = internalPath.substring(prefix.length);
    }

    // Ensure internalPath starts with / and ends without /
    if (!internalPath.startsWith("/")) internalPath = "/" + internalPath;
    const normalizedPath = internalPath.replace(/\/$/, "");
    trackerNavButtons.forEach((button) => {
      const buttonPath = button.getAttribute("data-path");
      const normalizedPath = internalPath.replace(/\/$/, "");

      if (
        buttonPath === normalizedPath ||
        (buttonPath === "/" && normalizedPath === "")
      ) {
        button.classList.add("active");
      } else {
        button.classList.remove("active");
      }
    });
  };

  // HTMX Listeners for Navigation Updates
  document.body.addEventListener("htmx:afterSwap", () => {
    setActiveTrackerNavButton(window.location.pathname);
  });

  document.body.addEventListener("htmx:historyCacheMiss", () => {
    setActiveTrackerNavButton(window.location.pathname);
  });

  // Initial check
  setActiveTrackerNavButton(window.location.pathname);
});

// --- Custom Dialog Functions (Must be global for inline 'onclick') ---

// Variables must be scoped globally if used outside DOMContentLoaded
const dialog = document.getElementById("action-dialog");
const backdrop = document.getElementById("backdrop");

function showDialog() {
  if (!dialog || !backdrop) {
    console.error("Dialog or Backdrop element missing.");
    return;
  }
  backdrop.classList.add("blurred-content");

  dialog.show();
}

function hideDialog() {
  if (!dialog || !backdrop) {
    console.error("Dialog or Backdrop element missing.");
    return;
  }
  backdrop.classList.remove("blurred-content");

  dialog.close();
  dialog.textContent = "";

  console.log("Hiding dialog!");
}

// Backdrop Click Listener
if (backdrop) {
  backdrop.addEventListener("click", hideDialog);
}

// --- Progress Bar Countdown Logic ---

// Function to start the progress bar countdown
function startProgressCountdown(progressElement, dialogElement, durationMs) {
  const totalDurationMs = durationMs || 5000;
  const intervalTimeMs = 10;
  const maxVal = totalDurationMs / intervalTimeMs;

  let currentValue = maxVal;
  progressElement.max = maxVal;
  progressElement.value = maxVal;

  // Clear any existing timer on this element
  if (progressElement.countdownTimer) {
    clearInterval(progressElement.countdownTimer);
  }

  const interval = setInterval(() => {
    currentValue -= 1;

    if (currentValue < 0) {
      clearInterval(interval);
      progressElement.value = 0;

      if (dialogElement && dialogElement.open) {
        dialogElement.close();
        dialogElement.textContent = "";
      }
      return;
    }

    progressElement.value = currentValue;
  }, intervalTimeMs);

  // Store the timer ID on the element for easy clearing if needed
  progressElement.countdownTimer = interval;
}

function showModal(modal, durationMs) {
  modal.show();

  const btn = document.getElementById("confirm-btn");
  if (btn) {
    btn.focus();
  }

  const progressElement = modal.querySelector("#countdown-progress");
  if (progressElement) {
    startProgressCountdown(progressElement, modal, durationMs);
  }
}

// Function to safely hide the error dialog (called by the manual Close button)
function hideModal() {
  const dialog = document.getElementById("modal");
  if (dialog) {
    // Find the progress bar and clear its timer if running
    const progressElement = dialog.querySelector("#countdown-progress");
    if (progressElement && progressElement.countdownTimer) {
      clearInterval(progressElement.countdownTimer);
    }

    dialog.close();
    dialog.textContent = "";
  }
}
