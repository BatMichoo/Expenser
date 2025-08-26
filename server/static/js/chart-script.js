function getOptions() {
  const options = {
    responsive: true,
    maintainAspectRatio: false,
    // scales: {
    //   y: {
    //     beginAtZero: true,
    //     title: {
    //       display: true,
    //       text: "Amount ($)",
    //     },
    //   },
    //   x: {
    //     title: {
    //       display: true,
    //       text: "Utility Type",
    //     },
    //   },
    // },
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

  const chart = new Chart(ctx, {
    type: "bar",
    data: {
      labels: [],
      datasets: [
        {
          // label: "Total Amount ($)",
          data: 0,
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
          canvas.Chart.options.plugins.title.text = "No Results!";
          canvas.Chart.update();
        } else {
          createNewChart(canvas);
        }
        return;
      }

      const expensesData = apiData.reduce((acc, e) => {
        if (acc[e.UtilityType]) {
          acc[e.UtilityType].amount += e.Amount;
        } else {
          acc[e.UtilityType] = {
            name: e.UtilityType,
            amount: e.Amount,
          };
        }
        return acc;
      }, {});

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

      Object.keys(expensesData).forEach((k) => {
        labels.push(k);
        amounts.push(expensesData[k].amount);
        colors.push(COLORS[k]);
      });

      canvas.Chart.data.labels = labels;
      canvas.Chart.data.datasets[0].data = amounts;
      canvas.Chart.data.datasets[0].backgroundColor = colors;
      canvas.Chart.data.datasets[0].borderColor = colors;

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
updateChart();
attachSearchListener();
