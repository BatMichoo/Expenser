import { initChart } from "./chart-core.js";

const GROCERIES_CONFIG = {
  prefix: "groceries",
  title: "Groceries Expenses by Category",
  dataSetLabel: "Total Amount (Groceries)",
  typeProp: "CategoryName",
  dateProp: "PurchaseDate",
  amountProp: "Amount",
  secondaryDataset: {
    amountProp: "TotalDiscount",
    label: "Discount Saved",
    color: "rgba(46, 204, 113, 0.9)",
  },
};

initChart(GROCERIES_CONFIG);
