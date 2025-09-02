function getOptions() {
  const options = {
    responsive: true,
    maintainAspectRatio: false,
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
  };
  return options;
}

function createNewChart() {
  const canvas = document.getElementById("chart");
  const ctx = canvas.getContext("2d");

  const theme = localStorage.getItem("theme");
  const textColor = getComputedStyle(document.documentElement).getPropertyValue(
    `--tx-color-${theme}`,
  );

  Chart.defaults.color = textColor;

  const chart = new Chart(ctx, {
    type: "bar",
    data: {
      labels: [],
      datasets: [
        {
          // label: "Total Amount ($)",
          data: [],
          borderWidth: 1,
        },
      ],
    },
    options: getOptions(),
  });

  canvas.Chart = chart;

  return chart;
}

function updateChart() {
  const type = document.getElementById("type");
  const year = document.getElementById("year");
  const canvas = document.getElementById("chart");

  const queryString = `/home/chart/search?type=${type.value}&year=${year.value}`;

  fetch(queryString)
    .then((r) => r.json())
    .then((apiData) => {
      if (!apiData || apiData.length === 0) {
        if (canvas.Chart) {
          canvas.Chart.data.labels = [];
          canvas.Chart.data.datasets[0].data = [];
          canvas.Chart.data.datasets.label = "";
          canvas.Chart.options.plugins.title.text = "No Results!";
          canvas.Chart.update();
        } else {
          createNewChart(canvas);
        }
        return;
      }

      let expensesData = {};

      if (type.value == "") {
        expensesData = apiData.reduce((acc, e) => {
          if (acc[e.UtilityType]) {
            acc[e.UtilityType].amount += e.Amount;
          } else {
            acc[e.UtilityType] = {
              name: e.UtilityType,
              amount: e.Amount,
            };
          }
          return acc;
        }, expensesData);
      } else {
        expensesData = apiData.reduce((acc, e) => {
          if (acc[e.ExpenseDate]) {
            acc[e.ExpenseDate].amount += e.Amount;
          } else {
            const options = {
              day: "numeric",
              month: "long",
            };
            const displayDate = new Date(e.ExpenseDate).toLocaleDateString(
              "en-GB",
              options,
            );
            acc[e.ExpenseDate] = {
              name: displayDate,
              amount: e.Amount,
              type: e.UtilityType,
            };
          }
          return acc;
        }, expensesData);
      }

      const COLORS = {
        Water: "rgba(54, 162, 235, 0.6)",
        TV: "rgba(153, 102, 255, 0.6)",
        Electricity: "rgba(255, 206, 86, 0.6)",
        Gas: "rgba(255, 99, 132, 0.6)",
        Internet: "rgba(75, 192, 192, 0.6)",
        Waste: "rgba(255, 159, 64, 0.6)",
        Other: "rgba(51, 77, 51, 0.2)",
      };

      const labels = [];
      const amounts = [];
      const colors = [];
      let dataSetLabel = "";

      if (type.value == "") {
        Object.keys(expensesData).forEach((k) => {
          labels.push(k);
          amounts.push(expensesData[k].amount);
          colors.push(COLORS[k]);
        });
        dataSetLabel = "Total of each expense:";
      } else {
        Object.keys(expensesData).forEach((k) => {
          labels.push(expensesData[k].name);
          amounts.push(expensesData[k].amount);
          colors.push(COLORS[expensesData[k].type]);
        });
      }

      canvas.Chart.data.labels = labels;
      canvas.Chart.data.datasets[0].data = amounts;
      canvas.Chart.data.datasets[0].backgroundColor = colors;
      canvas.Chart.data.datasets[0].borderColor = colors;
      canvas.Chart.options.plugins.legend.labels.generateLabels = updateLabels;
      if (canvas.Chart.options.plugins.title.text === "No Results!") {
        canvas.Chart.options.plugins.title.text =
          "Home Expenses by Utility Type";
      }
      canvas.Chart.update();
    })
    .catch((error) => console.error("Error fetching data:", error));
}

function updateLabels(chart) {
  const theme = localStorage.getItem("theme");
  const legendTextColor = getComputedStyle(
    document.documentElement,
  ).getPropertyValue(`--tx-color-${theme}`);
  const data = chart.data.datasets[0].data;
  const labels = chart.data.labels;
  const colors = chart.data.datasets[0].backgroundColor;
  return labels.map((label, i) => ({
    text: label,
    fontColor: legendTextColor,
    fillStyle: colors[i],
    strokeStyle: colors[i],
    lineWidth: 1,
    hidden: chart.getDatasetMeta(0).data[i].hidden,
  }));
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
updateChart();
attachSearchListener();
