// examples/08_tsx_data_table.tsx
// Demonstrating practical peek usage in TSX: data shaping, formatters, and component logic.

interface Order {
  id: string;
  customer: string;
  items: number;
  total: number;
  status: "pending" | "shipped" | "delivered" | "refunded";
  createdAt: string;
}

const mockOrders: Order[] = [
  { id: "ORD-101", customer: "Alice Smith", items: 3, total: 149.99, status: "delivered", createdAt: "2026-09-20" },
  { id: "ORD-102", customer: "Bob Jones", items: 1, total: 34.50, status: "pending", createdAt: "2026-09-22" },
  { id: "ORD-103", customer: "Charlie Brown", items: 5, total: 420.00, status: "shipped", createdAt: "2026-09-21" },
  { id: "ORD-104", customer: "Diana Prince", items: 2, total: 89.00, status: "refunded", createdAt: "2026-09-19" },
];

// 1. Test data wrangling and aggregations before returning markup
const activeRevenue = mockOrders
  .filter((o) => o.status !== "refunded")
  .reduce((sum, o) => sum + o.total, 0);
activeRevenue;

const statusCounts = mockOrders.reduce<Record<string, number>>((acc, o) => {
  acc[o.status] = (acc[o.status] || 0) + 1;
  return acc;
}, {});
statusCounts;

// 2. Pure Helper Formatters
function formatCurrency(amount: number): string {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" }).format(amount);
}
formatCurrency(149.99);

function getStatusBadgeClass(status: Order["status"]): string {
  const map: Record<Order["status"], string> = {
    pending: "badge-warning",
    shipped: "badge-info",
    delivered: "badge-success",
    refunded: "badge-danger",
  };
  return map[status] || "badge-default";
}
getStatusBadgeClass("delivered");
getStatusBadgeClass("pending");

// 3. UI Component Definition
function OrderRow({ order }: { order: Order }) {
  return (
    <tr key={order.id} className="order-row">
      <td className="font-mono">{order.id}</td>
      <td>{order.customer}</td>
      <td>{order.items} items</td>
      <td className="text-right font-bold">{formatCurrency(order.total)}</td>
      <td>
        <span className={`badge ${getStatusBadgeClass(order.status)}`}>
          {order.status.toUpperCase()}
        </span>
      </td>
    </tr>
  );
}

// 4. Element Instantiation and Props Inspection
const singleRow = <OrderRow order={mockOrders[0]} />;
singleRow.props.order.customer;
singleRow.props.order.total;

// 5. Mapping lists of JSX components
const tableRows = mockOrders.map((order) => <OrderRow key={order.id} order={order} />);
tableRows.length;
tableRows[0].props.order.id;

export {};
