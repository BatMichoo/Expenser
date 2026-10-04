const DEFAULT_COLORS = {
  Water: "rgba(54, 162, 235, 0.6)",
  TV: "rgba(153, 102, 255, 0.6)",
  Electricity: "rgba(255, 206, 86, 0.6)",
  Gas: "rgba(255, 99, 132, 0.6)",
  Internet: "rgba(75, 192, 192, 0.6)",
  Waste: "rgba(255, 159, 64, 0.6)",
  Other: "rgba(51, 77, 51, 0.2)",
};

// Aggregation functions: each takes the raw rows fetched from the server plus
// context describing which fields to read, and returns { labels, amounts, colorKeys }
// ready to be dropped into the chart's dataset.
const AGG_FUNCTIONS = {
  default(rows, ctx) {
    const { typeValue, typeProp, dateProp, amountProp, secondaryAmountProp } = ctx;
    const buckets = {};

    if (typeValue === "") {
      rows.forEach((r) => {
        const key = r[typeProp];
        if (!buckets[key]) {
          buckets[key] = { name: key, amount: 0, colorKey: key };
          if (secondaryAmountProp) buckets[key].secondaryAmount = 0;
        }
        buckets[key].amount += r[amountProp];
        if (secondaryAmountProp) buckets[key].secondaryAmount += r[secondaryAmountProp];
      });
    } else {
      rows.forEach((r) => {
        const key = r[dateProp];
        if (!buckets[key]) {
          const displayDate = new Date(r[dateProp]).toLocaleDateString(
            "en-GB",
            { day: "numeric", month: "long" },
          );
          buckets[key] = {
            name: displayDate,
            amount: 0,
            colorKey: r[typeProp],
          };
          if (secondaryAmountProp) buckets[key].secondaryAmount = 0;
        }
        buckets[key].amount += r[amountProp];
        if (secondaryAmountProp) buckets[key].secondaryAmount += r[secondaryAmountProp];
      });
    }

    return bucketsToSeries(buckets, Object.keys(buckets));
  },

  totalPerYear(rows, ctx) {
    const { dateProp, amountProp, typeProp, secondaryAmountProp } = ctx;
    const commonColorKey = rows.length ? rows[0][typeProp] : null;
    const buckets = {};

    rows.forEach((r) => {
      const year = new Date(r[dateProp]).getFullYear();
      if (!buckets[year]) {
        buckets[year] = { name: String(year), amount: 0, colorKey: commonColorKey };
        if (secondaryAmountProp) buckets[year].secondaryAmount = 0;
      }
      buckets[year].amount += r[amountProp];
      if (secondaryAmountProp) buckets[year].secondaryAmount += r[secondaryAmountProp];
    });

    return bucketsToSeries(buckets, Object.keys(buckets).sort());
  },

  maxMonthPerYear(rows, ctx) {
    return monthlyAggregatePerYear(rows, ctx, (monthSums) => Math.max(...monthSums));
  },

  avgMonthPerYear(rows, ctx) {
    return monthlyAggregatePerYear(
      rows,
      ctx,
      (monthSums) => monthSums.reduce((a, b) => a + b, 0) / monthSums.length,
    );
  },
};

function monthlyAggregatePerYear(rows, ctx, reduceMonths) {
  const { dateProp, amountProp, typeProp } = ctx;
  const commonColorKey = rows.length ? rows[0][typeProp] : null;
  const yearMonths = {};

  rows.forEach((r) => {
    const d = new Date(r[dateProp]);
    const year = d.getFullYear();
    const month = d.getMonth();
    yearMonths[year] = yearMonths[year] || {};
    yearMonths[year][month] = (yearMonths[year][month] || 0) + r[amountProp];
  });

  const buckets = {};
  Object.keys(yearMonths).forEach((year) => {
    const monthSums = Object.values(yearMonths[year]);
    buckets[year] = {
      name: year,
      amount: reduceMonths(monthSums),
      colorKey: commonColorKey,
    };
  });

  return bucketsToSeries(buckets, Object.keys(buckets).sort());
}

function bucketsToSeries(buckets, orderedKeys) {
  const labels = [];
  const amounts = [];
  const colorKeys = [];
  const hasSecondary = orderedKeys.some((key) => buckets[key].secondaryAmount !== undefined);
  const secondaryAmounts = hasSecondary ? [] : undefined;
  orderedKeys.forEach((key) => {
    labels.push(buckets[key].name);
    amounts.push(buckets[key].amount);
    colorKeys.push(buckets[key].colorKey);
    if (hasSecondary) secondaryAmounts.push(buckets[key].secondaryAmount || 0);
  });
  return { labels, amounts, colorKeys, secondaryAmounts };
}

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

  if (theme === "dark") {
    textColor = "#9eaece";
  }

  const datasets = [
    {
      label: config.dataSetLabel || "Total Amount ($)",
      data: [],
      borderWidth: 1,
    },
  ];

  if (config.secondaryDataset) {
    datasets.push({
      type: "line",
      label: config.secondaryDataset.label,
      data: [],
      borderColor: config.secondaryDataset.color,
      backgroundColor: config.secondaryDataset.color,
      borderWidth: 2,
      tension: 0.3,
      order: 0,
    });
  }

  const chart = new Chart(ctx, {
    type: "bar",
    data: {
      labels: [],
      datasets,
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
  const { prefix, colors: customColors, typeProp, dateProp } = config;
  const amountProp = config.amountProp || "Amount";
  const COLORS = { ...DEFAULT_COLORS, ...customColors };

  const type = document.getElementById("type");
  const year = document.getElementById("year");
  const fn = document.getElementById("agg-function");
  const canvas = document.getElementById("chart");

  const fnValue = fn ? fn.value : "default";
  const usesAllYears = fnValue !== "default";
  if (year) {
    year.disabled = usesAllYears;
  }

  const yearParam = usesAllYears ? "" : year.value;
  const queryString = `/${prefix}/chart/search?type=${type.value}&year=${yearParam}`;

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

      const aggFn = AGG_FUNCTIONS[fnValue] || AGG_FUNCTIONS.default;
      const { labels, amounts, colorKeys, secondaryAmounts } = aggFn(apiData, {
        typeValue: type.value,
        typeProp,
        dateProp,
        amountProp,
        secondaryAmountProp: config.secondaryDataset && config.secondaryDataset.amountProp,
      });

      const colors = colorKeys.map((key) => COLORS[key] || COLORS.Other);

      canvas.Chart.data.labels = labels;
      canvas.Chart.data.datasets[0].data = amounts;
      canvas.Chart.data.datasets[0].backgroundColor = colors;
      canvas.Chart.data.datasets[0].borderColor = colors;
      if (config.secondaryDataset) {
        canvas.Chart.data.datasets[1].data = secondaryAmounts || [];
      }
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
  const items = labels.map((label, i) => ({
    text: label,
    fontColor: textColor,
    fillStyle: colors[i],
    strokeStyle: colors[i],
    lineWidth: 1,
    hidden: chart.getDatasetMeta(0).data[i].hidden,
  }));

  if (config.secondaryDataset && chart.data.datasets[1] && chart.data.datasets[1].data.length) {
    items.push({
      text: config.secondaryDataset.label,
      fontColor: textColor,
      fillStyle: config.secondaryDataset.color,
      strokeStyle: config.secondaryDataset.color,
      lineWidth: 2,
      datasetIndex: 1,
      hidden: !chart.isDatasetVisible(1),
    });
  }

  return items;
}

function attachSearchListener(buttonId, updateFunc) {
  document.body.addEventListener("htmx:afterSettle", () => {
    const newBtn = document.getElementById(`${buttonId}-chart-search`);
    const fnSelect = document.getElementById("agg-function");
    const canvas = document.getElementById("chart");
    if (newBtn) {
      if (newBtn.updateChartListener) {
        newBtn.removeEventListener("click", newBtn.updateChartListener);
      }
      newBtn.addEventListener("click", updateFunc);
      newBtn.updateChartListener = updateFunc;

      if (fnSelect && !fnSelect.updateChartListener) {
        fnSelect.addEventListener("change", updateFunc);
        fnSelect.updateChartListener = updateFunc;
      }

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
