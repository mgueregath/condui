import lightLogo from "../../assets/images/condui_light_theme.png";
import darkLogo from "../../assets/images/condui_dark_theme.png";
import lightHorizontal from "../../assets/images/condui_horizonta_light_theme.png";
import darkHorizontal from "../../assets/images/condui_horizontal_dark_Theme.png";
import "./ConduiLogo.css";

export default function ConduiLogo({ horizontal = false, className = "", style }) {
  return (
    <>
      <img className={`condui-logo-light ${className}`} src={horizontal ? lightHorizontal : lightLogo} alt="Condui" style={style} />
      <img className={`condui-logo-dark ${className}`} src={horizontal ? darkHorizontal : darkLogo} alt="Condui" style={style} />
    </>
  );
}
