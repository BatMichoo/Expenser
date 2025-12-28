const DEFAULT_COLORS = {
  Water: "rgba(54, 162, 235, 0.6)",
  TV: "rgba(153, 102, 255, 0.6)",
  Electricity: "rgba(255, 206, 86, 0.6)",
  Gas: "rgba(255, 99, 132, 0.6)",
  Internet: "rgba(75, 192, 192, 0.6)",
  Waste: "rgba(255, 159, 64, 0.6)",
  Other: "rgba(51, 77, 51, 0.2)",
};

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

function createNewChart(config = {}) {
  const canvas = document.getElementById("chart");
  const ctx = canvas.getContext("2d");

  let textColor = "#3a4763";

  const theme = localStorage.getItem("theme");
  // const textColor = getComputedStyle(document.documentElement).getPropertyValue(
  //   `--text-muted`,
  // );

  if (theme === "dark") {
    textColor = "#9eaece";
  }

  console.log(textColor);

  // Chart.defaults.color = textColor;
  // Chart.defaults.plugins.legend.labels.color = textColor;

  const chart = new Chart(ctx, {
    type: "bar",
    data: {
      labels: [],
      datasets: [
        {
          label: config.dataSetLabel || "Total Amount ($)",
          data: [],
          borderWidth: 1,
        },
      ],
    },
    options: getOptions(),
  });

  canvas.Chart = chart;
  if (config.title) {
    chart.options.plugins.title.text = config.title;
  }

  return chart;
}

function updateChart(config) {
  const { prefix, colors: customColors } = config;
  const COLORS = { ...DEFAULT_COLORS, ...customColors };

  const type = document.getElementById("type");
  const year = document.getElementById("year");
  const canvas = document.getElementById("chart");

  const queryString = `${prefix}/chart/search?type=${type.value}&year=${year.value}`;

  fetch(queryString)
    .then((r) => r.json())
    .then((apiData) => {
      if (!apiData || apiData.length === 0) {
        canvas.Chart.data.labels = [];
        canvas.Chart.data.datasets[0].data = [];
        canvas.Chart.data.datasets[0].label = "";
        canvas.Chart.options.plugins.title.text = "No Results!";
        canvas.Chart.update();
        return;
      }

      let expensesData = {};
      // TODO: standartize prop names

      let typePropName = "UtilityType";
      if (prefix === "car") {
        typePropName = "Type";
      }

      let datePropName = "ExpenseDate";
      if (prefix === "car") {
        datePropName = "Date";
      }

      if (type.value == "") {
        expensesData = apiData.reduce((acc, e) => {
          if (acc[e[typePropName]]) {
            acc[e[typePropName]].amount += e.Amount;
          } else {
            acc[e[typePropName]] = {
              name: e[typePropName],
              amount: e.Amount,
            };
          }
          return acc;
        }, expensesData);
      } else {
        expensesData = apiData.reduce((acc, e) => {
          if (acc[e[datePropName]]) {
            acc[e[datePropName]].amount += e.Amount;
          } else {
            const options = {
              day: "numeric",
              month: "long",
            };
            const displayDate = new Date(e[datePropName]).toLocaleDateString(
              "en-GB",
              options,
            );
            acc[e[datePropName]] = {
              name: displayDate,
              amount: e.Amount,
              type: e[typePropName],
            };
          }
          return acc;
        }, expensesData);
      }

      const labels = [];
      const amounts = [];
      const colors = [];

      if (type.value == "") {
        Object.keys(expensesData).forEach((k) => {
          labels.push(k);
          amounts.push(expensesData[k].amount);
          colors.push(COLORS[k] || COLORS.Other);
        });
      } else {
        Object.keys(expensesData).forEach((k) => {
          labels.push(expensesData[k].name);
          amounts.push(expensesData[k].amount);
          colors.push(COLORS[expensesData[k].type] || COLORS.Other);
        });
      }

      canvas.Chart.data.labels = labels;
      canvas.Chart.data.datasets[0].data = amounts;
      canvas.Chart.data.datasets[0].backgroundColor = colors;
      canvas.Chart.data.datasets[0].borderColor = colors;
      canvas.Chart.options.plugins.legend.labels.generateLabels = (chart) =>
        updateLabels(chart, config);

      if (
        canvas.Chart.options.plugins.title.text === "No Results!" &&
        config.title
      ) {
        canvas.Chart.options.plugins.title.text = config.title;
      }
      canvas.Chart.update();
    })
    .catch((error) => console.error("Error fetching data:", error));
}

function updateLabels(chart, config) {
  const textColor = getComputedStyle(document.documentElement).getPropertyValue(
    `--text-muted`,
  );
  const data = chart.data.datasets[0].data;
  const labels = chart.data.labels;
  const colors = chart.data.datasets[0].backgroundColor;
  return labels.map((label, i) => ({
    text: label,
    fontColor: textColor,
    fillStyle: colors[i],
    strokeStyle: colors[i],
    lineWidth: 1,
    hidden: chart.getDatasetMeta(0).data[i].hidden,
  }));
}

function attachSearchListener(buttonId, updateFunc) {
  document.body.addEventListener("htmx:afterSettle", () => {
    const newBtn = document.getElementById(`${buttonId}-chart-search`);
    const canvas = document.getElementById("chart");
    if (newBtn) {
      if (newBtn.updateChartListener) {
        newBtn.removeEventListener("click", newBtn.updateChartListener);
      }
      newBtn.addEventListener("click", updateFunc);
      newBtn.updateChartListener = updateFunc;
      if (!canvas.Chart) {
        createNewChart({});
        updateFunc();
      }
    }
  });
}

export function initChart(config) {
  const specificUpdateChart = () => updateChart(config);

  createNewChart(config);
  specificUpdateChart();
  attachSearchListener(config.prefix, specificUpdateChart);
}
