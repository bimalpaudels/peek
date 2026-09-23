
interface UserBadgeProps {
  name: string;
  role: "admin" | "member";
  unreadCount?: number;
}

function UserBadge({ name, role, unreadCount = 0 }: UserBadgeProps) {
  return (
    <div className={`badge badge-${role}`}>
      <span className="username">{name}</span>
      {unreadCount > 0 && <span className="counter">({unreadCount})</span>}
    </div>
  );
}

// 1. Evaluate component invocation / JSX element directly
const element = <UserBadge name="Alice" role="admin" unreadCount={5} />;
element;

// 2. Inspect element properties and children
element.props.name;
element.props.role;

// 3. Test list of JSX elements
const team = ["Alice", "Bob", "Charlie"];
const badges = team.map((member, i) => (
  <UserBadge key={member} name={member} role="member" unreadCount={i} />
));
badges.length;
badges[0].props;
