function createNewChart() {
  const canvas = document.getElementById("chart");
  const ctx = canvas.getContext("2d");

  const chart = new Chart(ctx, {
    type: "pie",
    data: {
      labels: [],
      datasets: [
        {
          label: "Total Amount ($)",
          data: [],
          backgroundColor: [
            "rgba(255, 99, 132, 0.6)",
            "rgba(54, 162, 235, 0.6)",
            "rgba(255, 206, 86, 0.6)",
            "rgba(75, 192, 192, 0.6)",
            "rgba(153, 102, 255, 0.6)",
            "rgba(255, 159, 64, 0.6)",
          ],
          borderColor: [
            "rgba(255, 99, 132, 1)",
            "rgba(54, 162, 235, 1)",
            "rgba(255, 206, 86, 1)",
            "rgba(75, 192, 192, 1)",
            "rgba(153, 102, 255, 1)",
            "rgba(255, 159, 64, 1)",
          ],
          borderWidth: 1,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        y: {
          beginAtZero: true,
          title: {
            display: true,
            text: "Amount ($)",
          },
        },
        x: {
          title: {
            display: true,
            text: "Utility Type",
          },
        },
      },
      plugins: {
        legend: {
          display: true,
          position: "bottom",
        },
        title: {
          display: true,
          text: "No Results!",
        },
      },
    },
  });

  canvas.Chart = chart;

  return chart;
}

function updateChart() {
  const fromDate = document.getElementById("from");
  const toDate = document.getElementById("to");
  const canvas = document.getElementById("chart");

  fetch("/home/chart/search" + `?from=${fromDate.value}&to=${toDate.value}`)
    .then((r) => r.json())
    .then((apiData) => {
      if (!apiData || apiData.length === 0) {
        if (canvas.Chart) {
          canvas.Chart.data.labels = [];
          canvas.Chart.data.datasets[0].data = [];
          canvas.Chart.options.plugins.title.text = "No Results!";
          canvas.Chart.update();
        } else {
          createNewChart(canvas);
        }
        return;
      }

      const labels = apiData.map((item) => item.UtilityType);
      const amounts = apiData.map((item) => item.Amount);

      canvas.Chart.data.labels = labels;
      canvas.Chart.data.datasets[0].data = amounts;

      if ((canvas.Chart.options.plugins.title.text = "No Results!")) {
        canvas.Chart.options.plugins.title.text =
          "Home Expenses by Utility Type";
      }
      canvas.Chart.update();
    })
    .catch((error) => console.error("Error fetching data:", error));
}

function attachSearchListener() {
  document.body.addEventListener("htmx:afterSettle", (event) => {
    const newBtn = document.getElementById("chart-search");
    if (newBtn) {
      newBtn.addEventListener("click", updateChart);
    }
  });
}

createNewChart();
attachSearchListener();
