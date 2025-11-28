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
    trackerNavButtons.forEach((button) => {
      const buttonPath = button.getAttribute("data-path");
      const normalizedPath = path.replace(/\/$/, "");

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

function showCustomDialog() {
  if (!dialog || !backdrop) {
    console.error("Dialog or Backdrop element missing.");
    return;
  }
  backdrop.classList.add("blurred-content");

  dialog.show();
}

function hideCustomDialog() {
  if (!dialog || !backdrop) {
    console.error("Dialog or Backdrop element missing.");
    return;
  }
  backdrop.classList.remove("blurred-content");

  dialog.close();
  dialog.textContent = "";
}

// Backdrop Click Listener
if (backdrop) {
  backdrop.addEventListener("click", hideCustomDialog);
}

// --- Progress Bar Countdown Logic ---

// Function to start the progress bar countdown
function startProgressCountdown(progressElement, durationMs, dialogElement) {
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

function initializeDialogTimer(dialogElement, durationMs) {
  const progressElement = dialogElement.querySelector("#countdown-progress");

  if (progressElement) {
    startProgressCountdown(progressElement, durationMs, dialogElement);
  }
}

// Function to safely hide the error dialog (called by the manual Close button)
function hideErrorDialog() {
  const dialog = document.getElementById("error-modal");
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
