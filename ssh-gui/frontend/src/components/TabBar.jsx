export default function TabBar({
  tabs,
  activeTab,
  onSelect,
}) {

  return (
    <div className="tabbar">

      {tabs.map(tab => (

        <div
          key={tab.id}
          className={
            activeTab === tab.id
              ? "tab active"
              : "tab"
          }
          onClick={() => onSelect(tab.id)}
        >
          {tab.title}
        </div>

      ))}

    </div>
  );
}
