import { initChart } from "./chart-core.js";

const CAR_COLORS = {
  Fuel: "rgba(255, 0, 0, 0.8)",
  Insurance: "rgba(0, 100, 255, 0.8)",
  "Maintenance/Repair": "rgba(0, 255, 0, 0.8)",
  "Parking/Tolls": "rgba(255, 165, 0, 0.8)",
  Tires: "rgb(0 0 0 / 80%)",
  Oil: "rgb(170 75 0 / 71%)",
  Other: "rgb(51 51 51 / 90%)",
  "Car Wash": "rgb(255 255 255 / 80%)",
};

const CAR_CONFIG = {
  prefix: "car",
  title: "Car Expenses by Type",
  colors: CAR_COLORS,
  dataSetLabel: "Total Amount (Car)",
};

initChart(CAR_CONFIG);
