package libbox

type FDroidMirror struct {
	URL     string
	Country string
	Name    string
}

type FDroidMirrorIterator interface {
	Len() int32
	HasNext() bool
	Next() *FDroidMirror
}

var builtinFDroidMirrors = []FDroidMirror{
	// Official
	{URL: "https://f-droid.org/repo", Country: "Official", Name: "f-droid.org"},
	{URL: "https://cloudflare.f-droid.org/repo", Country: "Official", Name: "Cloudflare CDN"},

	// China
	{URL: "https://mirrors.tuna.tsinghua.edu.cn/fdroid/repo", Country: "China", Name: "Tsinghua TUNA"},
	{URL: "https://mirrors.nju.edu.cn/fdroid/repo", Country: "China", Name: "Nanjing University"},
	{URL: "https://mirror.iscas.ac.cn/fdroid/repo", Country: "China", Name: "ISCAS"},
	{URL: "https://mirror.nyist.edu.cn/fdroid/repo", Country: "China", Name: "NYIST"},
	{URL: "https://mirrors.cqupt.edu.cn/fdroid/repo", Country: "China", Name: "CQUPT"},
	{URL: "https://mirrors.shanghaitech.edu.cn/fdroid/repo", Country: "China", Name: "ShanghaiTech"},

	// India
	{URL: "https://mirror.hyd.albony.in/fdroid/repo", Country: "India", Name: "Albony Hyderabad"},
	{URL: "https://mirror.del2.albony.in/fdroid/repo", Country: "India", Name: "Albony Delhi"},

	// Taiwan
	{URL: "https://mirror.ossplanet.net/fdroid/repo", Country: "Taiwan", Name: "OSSPlanet"},

	// France
	{URL: "https://fdroid.tetaneutral.net/fdroid/repo", Country: "France", Name: "tetaneutral.net"},
	{URL: "https://mirror.freedif.org/fdroid/repo", Country: "France", Name: "FreeDif"},

	// Germany
	{URL: "https://ftp.fau.de/fdroid/repo", Country: "Germany", Name: "FAU Erlangen"},
	{URL: "https://ftp.agdsn.de/fdroid/repo", Country: "Germany", Name: "AGDSN Dresden"},
	{URL: "https://ftp.gwdg.de/pub/android/fdroid/repo", Country: "Germany", Name: "GWDG"},
	{URL: "https://mirror.level66.network/fdroid/repo", Country: "Germany", Name: "Level66"},
	{URL: "https://mirror.mci-1.serverforge.org/fdroid/repo", Country: "Germany", Name: "ServerForge"},

	// Netherlands
	{URL: "https://ftp.snt.utwente.nl/pub/software/fdroid/repo", Country: "Netherlands", Name: "University of Twente"},

	// Sweden
	{URL: "https://ftp.lysator.liu.se/pub/fdroid/repo", Country: "Sweden", Name: "Lysator"},

	// Denmark
	{URL: "https://mirrors.dotsrc.org/fdroid/repo", Country: "Denmark", Name: "dotsrc.org"},

	// Austria
	{URL: "https://mirror.kumi.systems/fdroid/repo", Country: "Austria", Name: "Kumi Systems"},

	// Switzerland
	{URL: "https://mirror.init7.net/fdroid/repo", Country: "Switzerland", Name: "Init7"},

	// Romania
	{URL: "https://mirrors.hostico.ro/fdroid/repo", Country: "Romania", Name: "Hostico"},
	{URL: "https://mirrors.chroot.ro/fdroid/repo", Country: "Romania", Name: "Chroot"},
	{URL: "https://ftp.lug.ro/fdroid/repo", Country: "Romania", Name: "LUG Romania"},

	// US
	{URL: "https://plug-mirror.rcac.purdue.edu/fdroid/repo", Country: "US", Name: "Purdue"},
	{URL: "https://mirror.fcix.net/fdroid/repo", Country: "US", Name: "FCIX"},
	{URL: "https://opencolo.mm.fcix.net/fdroid/repo", Country: "US", Name: "OpenColo"},
	{URL: "https://forksystems.mm.fcix.net/fdroid/repo", Country: "US", Name: "Fork Systems"},
	{URL: "https://southfront.mm.fcix.net/fdroid/repo", Country: "US", Name: "South Front"},
	{URL: "https://ziply.mm.fcix.net/fdroid/repo", Country: "US", Name: "Ziply"},

	// Canada
	{URL: "https://mirror.quantum5.ca/fdroid/repo", Country: "Canada", Name: "Quantum5"},

	// Australia
	{URL: "https://mirror.aarnet.edu.au/fdroid/repo", Country: "Australia", Name: "AARNet"},

	// Other
	{URL: "https://mirror.cyberbits.eu/fdroid/repo", Country: "Europe", Name: "Cyberbits EU"},
	{URL: "https://mirror.eu.ossplanet.net/fdroid/repo", Country: "Europe", Name: "OSSPlanet EU"},
	{URL: "https://mirror.cyberbits.asia/fdroid/repo", Country: "Asia", Name: "Cyberbits Asia"},
	{URL: "https://mirrors.jevincanders.net/fdroid/repo", Country: "US", Name: "Jevincanders"},
	{URL: "https://mirrors.komogoto.com/fdroid/repo", Country: "US", Name: "Komogoto"},
	{URL: "https://fdroid.rasp.sh/fdroid/repo", Country: "Europe", Name: "rasp.sh"},
	{URL: "https://mirror.gofoss.xyz/fdroid/repo", Country: "Europe", Name: "GoFOSS"},
}

func GetFDroidMirrors() FDroidMirrorIterator {
	return newPtrIterator(builtinFDroidMirrors)
}
