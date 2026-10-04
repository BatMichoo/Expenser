import { initChart } from "./chart-core.js";

const HOUSE_CONFIG = {
  prefix: "house",
  title: "Home Expenses by Utility Type",
  typeProp: "UtilityType",
  dateProp: "ExpenseDate",
  amountProp: "Amount",
};

initChart(HOUSE_CONFIG);
