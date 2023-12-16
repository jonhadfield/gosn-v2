package items

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/stretchr/testify/require"
)

var (
	sInput = auth.SignInInput{
		Email:     os.Getenv("SN_EMAIL"),
		Password:  os.Getenv("SN_PASSWORD"),
		APIServer: os.Getenv("SN_SERVER"),
		Debug:     true,
	}
	testParas = []string{
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Ut venenatis est sit amet lectus aliquam, ac rutrum nibh vulputate. Etiam vel nulla dapibus, lacinia neque et, porttitor elit. Nulla scelerisque elit sem, ac posuere est gravida dignissim. Fusce laoreet, enim gravida vehicula aliquam, tellus sem iaculis lorem, rutrum congue ex lectus ut quam. Cras sollicitudin turpis magna, et tempor elit dignissim eu. Etiam sed auctor leo. Sed semper consectetur purus, nec vehicula tellus tristique ac. Cras a quam et magna posuere varius vitae posuere sapien. Morbi tincidunt tellus eu metus laoreet, quis pulvinar sapien consectetur. Fusce nec viverra lectus, sit amet ullamcorper elit. Vestibulum vestibulum augue sem, vitae egestas ipsum fringilla sit amet. Nulla eget ante sit amet velit placerat gravida.",
		"Duis odio tortor, consequat egestas neque dictum, porttitor laoreet felis. Sed sed odio non orci dignissim vulputate. Praesent a scelerisque lectus. Phasellus sit amet vestibulum felis. Integer blandit, nulla eget tempor vestibulum, nisl dolor interdum eros, sed feugiat est augue sit amet eros. Suspendisse maximus tortor sodales dolor sagittis, vitae mattis est cursus. Etiam lobortis nunc non mi posuere, vel elementum massa congue. Aenean ut lectus vitae nisl scelerisque semper.",
		"Quisque pellentesque mauris ut tortor ultrices, eget posuere metus rhoncus. Aenean maximus ultricies mauris vel facilisis. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Curabitur hendrerit, ligula a sagittis condimentum, metus nibh sodales elit, sed rhoncus felis ipsum sit amet sem. Phasellus nec massa non felis suscipit dictum. Aenean dictum iaculis metus quis aliquam. Aenean suscipit mi vel nisi finibus rhoncus. Donec eleifend, massa in convallis mattis, justo eros euismod dui, sollicitudin imperdiet nibh lacus sit amet diam. Praesent eu mollis ligula. In quis nisi egestas, scelerisque ante vitae, dignissim nisi. Curabitur vel est eget purus porta malesuada.",
		"Duis tincidunt eros ligula, et tincidunt lacus scelerisque ac. Cras aliquam ultrices egestas. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculous mus. Nunc sapien est, imperdiet in cursus id, suscipit ac orci. Integer magna augue, accumsan quis massa rutrum, dictum posuere odio. Vivamus vitae efficitur enim. Donec posuere sapien sit amet turpis lacinia rutrum. Nulla porttitor lacinia justo quis consequat.",
		"Quisque blandit ultricies nisi eu dignissim. Mauris venenatis enim et posuere ornare. Phasellus facilisis libero ut elit consequat scelerisque. Vivamus facilisis, nibh eget hendrerit malesuada, velit tellus vehicula justo, id ultrices justo orci nec dui. Sed hendrerit fermentum pulvinar. Aenean at magna gravida, finibus ligula non, cursus felis. Quisque consectetur malesuada magna ut cursus. Nam aliquet felis aliquet lobortis pulvinar. Fusce vel ipsum felis. Maecenas sapien magna, feugiat vitae tristique sit amet, vehicula ac quam. Donec a consectetur lorem, id euismod augue. Suspendisse metus ipsum, bibendum efficitur tortor vitae, molestie suscipit nulla. Proin vel felis eget libero auctor pulvinar eget ac diam. Vivamus malesuada elementum lobortis. Mauris nibh enim, pharetra eu elit vel, sagittis pulvinar ante. Ut efficitur nunc at odio elementum, sed pretium ante porttitor.",
		"Nulla convallis a lectus quis efficitur. Aenean quis vestibulum enim. Nunc in mattis tortor. Nullam sit amet placerat ipsum. Aene an sagittis, elit non bibendum posuere, sapien libero eleifend nisl, quis iaculis urna tortor ut nibh. Fusce fringilla elit in pellentesque laoreet. Duis ornare semper sagittis. Curabitur efficitur quam ac erat venenatis bibendum. Curabitur et luctus nunc, eu euismod augue. Mauris magna enim, vulputate eget gravida a, vestibulum non massa. Pellentesque eget pulvinar nisl.",
		"Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Proin a venenatis felis, a posuere augue. Cras ultrices libero in lorem congue ultrices. Integer porttitor, urna non vehicula maximus, est tellus volutpat erat, id volutpat eros erat sit amet mi. Quisque faucibus maximus risus, in placerat mauris venenatis vitae. Ut placerat, risus eu suscipit cursus, velit magna rhoncus dui, eu condimentum mauris nisi in ligula. Interdum et malesuada fames ac ante ipsum primis in faucibus. Aliquam sed dictum lectus. Quisque malesuada sapien mattis, consectetur augue non, sodales arcu. Vivamus imperdiet leo et lacus bibendum, eu venenatis odio auctor. Donec vitae massa vitae nisi tristique faucibus. Curabitur nec pretium ex. Quisque at sapien ornare, mollis nulla eget, tristique ex.",
		"Fusce faucibus id nulla et ornare. Nunc a diam urna. Ut tortor urna, fringilla eu pellentesque in, consectetur vel neque. Suspendisse at eros nisi. Phasellus dui libero, maximus ut orci sit amet, accumsan semper velit. Aenean quis interdum dolor. Sed molestie urna vitae turpis lacinia commodo. Fusce ipsum massa, blandit et nunc at, vestibulum tincidunt orci. Donec venenatis lorem sed urna sodales maximus. Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Maecenas orci lorem, bibendum ullamcorper congue ac, vestibulum vel neque. Nulla ut venenatis ex. Nunc pellentesque eros at metus dapibus, ut ullamcorper elit maximus.",
		"Quisque dapibus diam arcu, mollis accumsan dui convallis sed. Pellentesque laoreet nibh eget diam varius rhoncus. Vestibulum luctus, magna rhoncus sollicitudin condimentum, nisi augue faucibus lacus, at eleifend turpis mi eu purus. Vivamus non nisl magna. Praesent bibendum suscipit felis. Sed mi lorem, fringilla at commodo ut, accumsan sed velit. Vestibulum interdum quis leo sed aliquam. In ut velit quis quam pretium mollis vitae non nunc. Praesent ut dolor mi. Nunc scelerisque mi id elit dignissim, id sodales ipsum tincidunt. Duis sit amet risus mi. Morbi ornare neque nunc, semper ornare orci dignissim in. Donec ipsum diam, scelerisque tempus ante et, scelerisque convallis lorem. Aliquam facilisis imperdiet viverra. Pellentesque interdum, elit in consectetur euismod, metus odio pretium lorem, sed imperdiet neque est eu orci. Nunc nec massa et quam porta dictum.",
		"Mauris finibus tempor tempus. Suspendisse imperdiet in tortor ac condimentum. Nullam elementum est eget massa ullamcorper elementum eu quis velit. Nullam sed ipsum id velit consequat commodo. Quisque cursus eget mi nec elementum. Donec vel hendrerit nunc. Nunc egestas felis quam, et tristique nulla congue eu. Fusce quis ex bibendum, luctus urna id, vehicula ipsum. Nunc blandit, nibh a commodo congue, orci eros feugiat tellus, sed euismod lectus mi dapibus lacus. Etiam ac metus vel neque imperdiet efficitur. Suspendisse mattis quam ut turpis posuere faucibus. Sed eleifend justo ultricies odio facilisis bibendum.",
		"Donec in arcu sed justo lobortis ornare eu vitae nulla. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Curabitur pellentesque urna ipsum, non hendrerit urna laoreet dignissim. Integer pellentesque lorem velit, vitae vulputate libero scelerisque ac. Curabitur accumsan cursus leo, at mattis elit mattis et. Phasellus consequat justo et dui faucibus sodales. Cras facilisis vehicula dignissim. Donec consequat tincidunt mi ut faucibus. Ut a massa ullamcorper, finibus diam sed, accumsan erat. Aenean vitae dolor eu ipsum cursus faucibus et condimentum metus. Pellentesque non est id nunc finibus porta et at eros. Cras sodales congue sollicitudin. Nunc ullamcorper tortor vitae tortor aliquam, vitae ultricies neque lobortis. In hac habitasse platea dictumst. Aenean sed fermentum neque, ut pulvinar sem.",
		"Maecenas dapibus semper turpis, vitae laoreet sem facilisis eget. Curabitur sollicitudin purus id congue tincidunt. Nulla vitae nisl eu orci vehicula molestie. Duis ut eros ac nisl finibus molestie. Duis sit amet tempus ipsum, quis consequat metus. Curabitur sed tortor suscipit, consequat erat eget, dapibus tortor. In tristique augue lacus, in ultrices ex scelerisque a. Suspendisse potenti.",
		"Vestibulum efficitur ullamcorper diam non accumsan. Suspendisse ac nisi sit amet orci laoreet imperdiet. Integer tempor sapien nec sollicitudin sodales. Proin euismod, lectus quis lobortis gravida, tellus ligula semper ex, at vehicula ante dolor eget mi. Quisque metus libero, fermentum sodales venenatis in, tristique ac lacus. Aenean sodales nibh a sem rutrum, vel elementum velit interdum. Proin vel lectus ut neque gravida eleifend. Fusce maximus ante ligula, vestibulum congue nulla molestie et.",
		"Donec varius nibh sed ligula feugiat placerat. Fusce dolor ex, malesuada nec convallis id, maximus ac est. Sed eu ex ullamcorper, sagittis velit vel, congue enim. Maecenas eu posuere lectus. Proin eu nisl consequat mi euismod laoreet. Donec quis neque dolor. Donec in nulla gravida, imperdiet mi et, viverra elit.",
		"Nam et risus leo. Pellentesque ut pretium sem. Mauris ac orci sit amet ex placerat commodo. Suspendisse potenti. Vestibulum eleifend convallis sapien, nec semper libero convallis eget. Vivamus vitae ligula et lectus gravida consectetur ut eget quam. In hac habitasse platea dictumst.",
		"Vestibulum pretium tellus ac ipsum fringilla iaculis. Curabitur volutpat sapien nunc, in luctus lacus ullamcorper non. Vestibulum auctor, dui et semper sodales, augue tellus rutrum tortor, eu iaculis leo arcu ornare nulla. Nullam a urna efficitur, blandit nunc sed, tincidunt odio. Morbi cursus eros eget mattis porta. Mauris ac pellentesque metus. Morbi sagittis lacus id euismod tempor. Mauris ultrices risus vel tellus consequat, et tincidunt ipsum volutpat.",
		"Donec pulvinar risus non tellus faucibus, quis tempor elit vestibulum. Pellentesque aliquet lorem sed eros fringilla luctus. Nulla et lacus eget lorem feugiat dignissim. Pellentesque ac velit lacus. Vestibulum a justo tristique, cursus nisl in, bibendum nulla. Nam eu tempor purus, et dapibus ante. Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		"Donec pellentesque tellus quis arcu semper, ut dignissim mauris gravida. Etiam vel lacus sagittis, rhoncus orci ut, semper velit. Curabitur felis massa, aliquam eu dolor vel, ornare efficitur turpis. Nulla id elit nec orci maximus aliquam ut laoreet ipsum. Sed quis posuere massa. Aliquam lobortis, est quis sagittis interdum, eros risus maximus elit, nec facilisis mi tortor eu nisi. Suspendisse malesuada eleifend sodales. Maecenas mollis mi tortor, sit amet rutrum dolor tincidunt ut. Morbi finibus dignissim porta.",
		"Sed hendrerit massa id molestie ultricies. Sed pretium vel risus dictum ullamcorper. Etiam molestie orci feugiat quam aliquam, in maximus ipsum sodales. Nunc bibendum est dolor, vel rhoncus orci feugiat sit amet. Integer dignissim risus ut mauris volutpat, ac hendrerit erat sodales. Sed accumsan ex ex. Ut varius augue vitae mauris aliquam elementum. Nulla id volutpat magna, in bibendum enim. Aliquam iaculis nunc et augue dapibus, sit amet dictum enim feugiat. Mauris nisl quam, viverra ac massa ac, suscipit porttitor risus.",
		"Donec elementum scelerisque leo, vitae interdum neque fringilla vel. Etiam eu leo rutrum, mollis sapien quis, bibendum mi. Quisque sem ex, tincidunt nec tincidunt molestie, varius eu tellus. Donec viverra sit amet purus eget tincidunt. Cras pulvinar porttitor tellus eu faucibus. In sit amet rhoncus sem. Mauris faucibus tortor urna, at faucibus quam volutpat ac. Vivamus ut molestie velit, quis tristique dolor. Duis molestie semper nisi ac feugiat. Nunc a nisi convallis, commodo dui et, ultrices velit.",
		"Nullam semper suscipit mi, ut consequat velit suscipit nec. Curabitur finibus pharetra diam sed condimentum. Aliquam dolor ligula, hendrerit nec pretium vitae, convallis sit amet leo. Vestibulum magna tortor, blandit eget fermentum in, porttitor et orci. In aliquam urna eu mollis lacinia. Nullam semper interdum orci. Cras semper mauris nec elementum mattis. Donec porta luctus ultricies. Pellentesque in luctus ligula. In ante ex, lacinia at dui vel, bibendum ornare lacus. Quisque porta eleifend dignissim. Nunc sed placerat risus. Etiam fermentum nec enim in dignissim.",
		"Nunc sodales tellus a urna cursus, ac posuere felis ultrices. Suspendisse maximus massa sem, in laoreet eros molestie ut. Aliquam suscipit vel orci at vestibulum. Fusce hendrerit, felis eu posuere sollicitudin, est velit dictum turpis, vitae faucibus mauris mauris ut velit. Praesent bibendum lectus ut vestibulum maximus. Donec blandit ligula libero, ut tempor nulla scelerisque posuere. Nulla egestas elit ex. Maecenas pretium semper quam in rhoncus. Fusce a viverra nunc, sed placerat libero. Sed venenatis convallis risus sed condimentum. Vivamus euismod tellus eu sagittis facilisis. Aliquam ac massa lacus. Interdum et malesuada fames ac ante ipsum primis in faucibus. Ut turpis ligula, euismod ac enim sed, porta tristique magna.",
		"Pellentesque venenatis elementum enim, nec consequat felis. Nulla facilities. Suspendisse tempus erat non ipsum vulputate, in lacinia elit feugiat. Maecenas et velit eget tortor congue luctus nec eu urna. Duis congue tellus vel purus convallis tristique. Aenean dapibus tincidunt leo, vel aliquam turpis pretium sit amet. Pellentesque mollis in massa eu mattis. Suspendisse potenti. Cras facilisis at purus ut elementum. Sed volutpat eget nisl id lobortis. Etiam vestibulum lectus id justo vulputate lacinia quis ut lectus. Phasellus tincidunt dolor id nisl placerat, a consequat est volutpat. Phasellus venenatis finibus ante, et eleifend justo pulvinar id. Cras eu ligula quis libero tempor condimentum. Praesent accumsan enim in sodales vulputate. Curabitur rhoncus ante a luctus interdum.",
		"Maecenas vel rhoncus turpis, sit amet varius lacus. Vivamus venenatis sapien vel mi euismod, eget commodo tortor auctor. Cras sit amet dictum quam, non fermentum massa. Aliquam gravida est sed gravida suscipit. Praesent finibus tempor magna, ut dapibus dolor. Proin quis pulvinar arcu. Suspendisse tempor sem justo, at dignissim justo elementum vel. Aenean vitae dolor varius lacus rutrum eleifend.",
		"Etiam ut varius enim. Quisque ligula neque, accumsan et neque eget, pretium lacinia nisi. Etiam aliquet id quam a ullamcorper. Fusce eleifend, mauris vitae placerat egestas, orci erat euismod enim, ut posuere nisl justo placerat libero. Nam ac dui ac lorem laoreet maximus. Curabitur risus leo, feugiat et ligula ac, pellentesque ullamcorper lorem. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae; Donec dignissim non turpis tristique hendrerit. Donec libero odio, ullamcorper condimentum tincidunt ac, hendrerit sed metus. Maecenas venenatis sodales ex. Vestibulum sit amet finibus urna, eu pellentesque velit. Donec accumsan lectus sit amet purus lacinia, et aliquam quam imperdiet. Nunc quis sem fermentum, consectetur urna quis, tristique eros. Sed at tortor a velit blandit aliquam in semper odio. Etiam laoreet sapien lacus, at convallis felis feugiat vitae. Integer et facilisis nibh.",
		"Nullam consequat vehicula euismod. Donec non metus sed nulla bibendum facilisis sed vitae orci. Donec at sapien elit. Sed luctus id augue a gravida. Quisque bibendum nisl sed imperdiet congue. Nam tristique diam diam, ut finibus ante laoreet sit amet. Fusce eget condimentum sem, eget imperdiet massa. Sed orci velit, aliquet a malesuada ac, convallis vitae elit. Aliquam molestie tellus vitae tellus accumsan, quis dapibus purus placerat. Cras commodo, ligula quis commodo congue, ipsum enim placerat nisi, eu congue ante dolor sed ante. Nunc luctus est id metus eleifend, sed consequat leo gravida. Phasellus mattis enim sit amet placerat vehicula. Suspendisse vestibulum lacus sit amet nunc placerat, et ultricies elit fermentum. In et est ac turpis vestibulum bibendum.",
		"Cras in est efficitur, volutpat nisi ut, faucibus nisl. Proin vehicula quam vel ante cursus, vel ultricies erat rutrum. Aliquam pharetra fringilla mauris eget condimentum. Sed placerat turpis eu turpis semper, nec vestibulum sem ornare. Suspendisse potenti. Ut at elit porta, luctus lorem in, tincidunt nunc. Vivamus in justo a lectus sollicitudin tempor non non tellus. Phasellus nec iaculis lacus. Cras consequat orci nec feugiat rutrum. Praesent vulputate, lorem a vulputate ornare, nisi ante tempus elit, sit amet interdum eros nunc vitae erat. Donec at neque in orci ultricies pulvinar eu sit amet quam. Duis tempus vitae sapien vel malesuada.",
		"In auctor condimentum diam sed auctor. Vivamus id rutrum arcu. Proin venenatis, neque malesuada eleifend posuere, urna est pellentesque turpis, vitae scelerisque nisi nunc sit amet ligula. Vestibulum viverra consequat est in lobortis. Nam efficitur arcu at neque lacinia, ut condimentum leo gravida. Vestibulum vitae nisi leo. Nulla odio dolor, malesuada a magna non, accumsan efficitur nisi. Proin vitae lacus facilisis erat interdum pulvinar. Praesent in egestas tortor.", "Integer in massa odio. Duis ex mi, varius nec nibh eget, sodales pellentesque sapien. Aenean pharetra eros vitae tellus volutpat euismod. Vestibulum mattis condimentum metus eget ullamcorper. Integer ut iaculis lorem. Fusce nec lobortis massa. Aliquam pellentesque placerat lorem in efficitur. In pharetra et ex sed mattis. Praesent eget nisi sit amet metus ullamcorper fermentum. Integer ante dolor, vestibulum id massa in, dictum porta nisi. Duis lacinia est vitae enim dignissim blandit. Integer non dapibus velit. Cras auctor eu felis sodales tempor.",
		"Fusce convallis facilisis lacus id lacinia. Nunc rhoncus vulputate consectetur. Ut malesuada malesuada mi, molestie luctus purus feugiat at. Integer eros mauris, porttitor eu tristique quis, gravida id enim. Nullam vitae lectus nulla. Etiam consequat sapien vitae risus volutpat, vitae condimentum nisl ultrices. Donec et magna sed justo suscipit iaculis. Vestibulum id mauris eu eros tempus interdum. Morbi porta libero quis leo consectetur maximus. Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Proin ut dui pharetra, faucibus augue blandit, tempor massa. Duis urna odio, luctus eget justo sed, consectetur faucibus mauris. Phasellus vel sapien justo. Donec sed lectus eget lectus ultrices ornare. Curabitur ultrices sapien libero, sit amet fermentum mi bibendum nec.",
		"Phasellus sed quam mi. Aenean eget sodales neque. Nam convallis lacus non justo blandit, non ornare mauris imperdiet. Nam mattis commodo turpis et lacinia. Duis ac maximus lorem. Suspendisse euismod dui at sodales accumsan. Suspendisse vulputate lobortis sapien, viverra vehicula felis blandit vitae. Fusce viverra ultrices felis sed egestas. Duis ex diam, rutrum nec maximus sit amet, imperdiet ac ex. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculous mus. Mauris scelerisque facilisis eros, sed maximus erat elementum et.",
		"In ultricies, felis eu aliquet tempor, velit ante finibus ipsum, ut sagittis lacus urna sit amet erat. Vestibulum porttitor gravida scelerisque. Fusce semper faucibus est placerat varius. Curabitur nec quam pellentesque, pharetra magna nec, pellentesque nunc. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec sagittis tincidunt justo, nec sagittis turpis vehicula eu. Duis ultricies nulla urna, vel efficitur purus feugiat a. Etiam molestie pharetra ex. Cras ut est faucibus, maximus orci eget, accumsan sapien.",
		"Nam tempor, libero eget iaculis commodo, mauris neque feugiat diam, vel lacinia ipsum velit ac lorem. In eget fringilla velit. Nullam mattis elit sem, tincidunt ultricies quam euismod ac. Sed sagittis mauris id orci consequat, a accumsan ligula commodo. Quisque a faucibus ligula, eget porttitor sem. Quisque sit amet sollicitudin sem. Vestibulum dignissim erat lacus, id molestie metus malesuada eget. Vivamus ac metus non dui sagittis rutrum. Curabitur quam quam, luctus at hendrerit quis, vehicula in libero. Fusce et sapien lorem.",
		"Integer ornare ex tempus libero hendrerit, ac maximus purus imperdiet. Nullam tincidunt ex et ipsum vulputate luctus. Nunc ac sodales mi. Integer nec velit leo. Fusce quis ante tortor. Donec congue purus sem. Sed eget velit vel lacus semper suscipit. Praesent id lacinia enim. Fusce luctus in leo sit amet porttitor. Nam non lorem in enim fermentum auctor. Morbi non pellentesque ex.",
		"Integer viverra leo at erat ultrices bibendum. Quisque sollicitudin mauris sapien, non ornare neque congue eget. Nulla tempus, est sed lobortis facilisis, sapien justo placerat mauris, a tincidunt risus turpis eu massa. Ut volutpat ut justo non aliquam. Quisque dignissim dapibus nunc, eget sollicitudin ipsum dignissim sit amet. Mauris sed magna ac nulla scelerisque sodales quis ultricies sapien. Maecenas semper tristique ex, et congue metus.",
		"Ut luctus ligula id tellus faucibus lobortis. Donec vel turpis purus. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nullam malesuada, leo id condimentum luctus, ante quam auctor velit, at laoreet justo augue ac mi. Maecenas pharetra elit eu dui accumsan tempor. Sed turpis orci, laoreet at mauris non, sollicitudin pharetra ante. Ut gravida rutrum porttitor. Sed velit diam, consequat eu tempus in, ullamcorper non erat. Integer vel dui felis. Donec pharetra ipsum turpis, a pharetra tortor ullamcorper a. Aliquam sed sapien gravida massa tincidunt mollis eget vitae est. Sed dapibus hendrerit tortor eu sollicitudin. Sed ac eleifend neque. Quisque blandit mi turpis, eget faucibus arcu porttitor non. Integer scelerisque metus felis, et porta ligula pretium sit amet.",
		"Sed dictum rutrum velit eleifend tristique. Suspendisse porta vestibulum ultrices. Etiam ac sagittis magna, eget porta ipsum. Praesent non tempus metus. Sed sollicitudin velit quis efficitur placerat. Cras congue nulla dolor, vitae fermentum augue convallis tempor. Aliquam dui erat, sollicitudin quis eros fermentum, ultrices sagittis arcu. Curabitur pharetra porta tortor non consequat. Proin gravida eget leo lacinia mollis. Morbi dui arcu, ornare scelerisque sapien a, feugiat efficitur erat. Vestibulum ac leo porta nisl consequat molestie.",
		"Curabitur auctor faucibus ornare. Aenean sit amet sollicitudin quam, non condimentum dui. In sagittis neque at elit porttitor posuere. Fusce et ex vel nisi mattis ultricies id a eros. Fusce suscipit, nisl et dignissim consectetur, ligula risus mattis ante, at dictum dolor augue sit amet nulla. Aliquam laoreet, libero at laoreet efficitur, justo dolor mollis ex, non venenatis lorem tortor nec sapien. Suspendisse vestibulum dolor id maximus volutpat. Pellentesque mollis orci dolor, nec tempor turpis dapibus ut. Maecenas commodo nisi sapien, nec iaculis nisl hendrerit et. Integer non purus interdum, ultricies nunc vitae, mollis est.",
		"Cras elementum magna in elit cursus ultrices. Aliquam id massa fringilla, tincidunt augue nec, hendrerit tellus. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae; Cras porttitor nibh a tortor consectetur tincidunt non quis tortor. Mauris in mollis ipsum. Mauris in mauris quam. Mauris ut nibh nisi.",
		"Sed rutrum quam at est finibus, a commodo neque tempor. Etiam bibendum mauris scelerisque tortor faucibus, in lacinia sapien bibendum. Morbi facilisis lorem vitae magna scelerisque, at egestas ante dignissim. Phasellus sit amet nunc sapien. Vivamus sit amet purus finibus, bibendum lorem sit amet, euismod eros. Vivamus consequat non nisi a tristique. Vestibulum ac nulla eu sapien euismod aliquam. Praesent porttitor urna mi.", "Cras porttitor eros vel varius iaculis. Mauris id nulla felis. Aenean scelerisque, ligula et interdum pretium, erat libero fringilla purus, eleifend dapibus elit nunc fermentum nulla. Pellentesque quis mauris hendrerit, luctus sem sed, ullamcorper sapien. Cras nec iaculis velit, at tempus dui. Nullam a eros in orci egestas pellentesque. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae;",
		"Cras quis vulputate diam. Donec sed placerat felis. Quisque sed auctor elit, id lacinia justo. Proin aliquam orci nec efficitur auctor. Pellentesque laoreet, metus nec ultricies hendrerit, est quam consectetur sem, in fermentum elit neque at velit. Ut pharetra sem congue, malesuada sapien in, consequat mi. Suspendisse magna lectus, pellentesque in nunc non, congue tristique nisi. Praesent id nibh vulputate ante tempor porttitor. Nunc sed lorem non dolor dignissim iaculis at vitae nulla. In at bibendum arcu. Phasellus vel mauris sed lorem pellentesque tristique.",
		"Phasellus sit amet lectus mi. Donec sit amet magna non arcu posuere pulvinar nec a erat. Suspendisse tellus mi, dictum ac accumsan vel, aliquet eget felis. Nulla porta vitae nunc et malesuada. Curabitur tempus porttitor magna sit amet mattis. Praesent pretium nisl maximus, pulvinar mi dignissim, rhoncus lacus. Maecenas in suscipit nisi. Sed non nisl eu nibh elementum imperdiet nec a ex.",
		"Aenean lorem tellus, tempor vitae ornare et, tempor non urna. Suspendisse finibus lectus gravida justo eleifend, ac feugiat augue ultrices. Integer semper ex nisl, ac tempor risus maximus eget. Pellentesque a tortor dignissim erat volutpat convallis ac vel mi. Nam ultricies, diam nec interdum bibendum, urna ipsum bibendum elit, vitae euismod est dolor et lacus. Fusce consectetur iaculis mauris eget mattis. Quisque bibendum nibh quis orci sagittis consectetur. Donec maximus consectetur ex sed fringilla. Interdum et malesuada fames ac ante ipsum primis in faucibus. Duis congue elit nisi, sit amet vehicula purus bibendum ut.",
		"Ut aliquet commodo mauris, quis suscipit urna mollis at. Fusce gravida nibh risus, et sollicitudin leo placerat eget. Maecenas tincidunt hendrerit justo, ac mattis massa commodo vitae. Vivamus neque quam, tincidunt et venenatis nec, convallis et ex. Etiam blandit tellus vitae mi pretium dapibus. Proin ac nibh ullamcorper, tristique nibh eu, venenatis nunc. Mauris non leo gravida, aliquam lacus non, venenatis lacus. Curabitur quis dolor risus. Nullam eros arcu, vehicula a orci nec, pretium pellentesque enim. Sed vestibulum lectus ex, imperdiet eleifend metus facilisis eget.",
		"Etiam nulla lectus, volutpat quis iaculis et, consectetur sit amet sapien. Mauris dictum posuere nibh et ultrices. Curabitur finibus, urna eget cursus volutpat, lectus arcu scelerisque turpis, in pretium lorem nulla quis purus. Quisque in pharetra tortor. Aenean malesuada imperdiet ex. Nulla maximus felis diam, a accumsan odio tristique ac. Quisque id metus quis justo vulputate convallis. In rutrum tortor eget justo molestie, vitae condimentum odio ultrices. Sed sodales mi at commodo commodo. Donec euismod, lectus eu euismod aliquet, lorem libero vehicula urna, id auctor leo tortor quis velit. Curabitur odio magna, iaculis vel viverra id, egestas id augue. Donec enim augue, congue nec maximus sit amet, auctor non mauris. Pellentesque fermentum, lectus vel auctor pulvinar, velit odio aliquam turpis, vel imperdiet est neque at turpis. Fusce ornare auctor justo, id malesuada mauris dictum sed. Pellentesque convallis, turpis eget vulputate egestas, elit nisi cursus lectus, vel tincidunt quam ipsum sit amet sapien.",
		"Pellentesque posuere ullamcorper velit eu interdum. Vivamus posuere aliquam velit a rhoncus. Fusce viverra fermentum justo id rhoncus. Quisque at dui dapibus, bibendum eros fermentum, laoreet ligula. Curabitur porta, diam at fringilla congue, dui enim bibendum elit, non scelerisque lorem risus eget lacus. Proin risus nibh, scelerisque nec feugiat vel, finibus interdum lorem. Morbi elit purus, fringilla at odio at, cursus condimentum erat. In tristique nisi eu ex tristique, sed bibendum est eleifend. Mauris interdum quis velit sit amet sodales. Integer semper tempus magna, vel vehicula eros pharetra sed. Sed tincidunt porttitor tellus. Suspendisse potenti. Nulla et scelerisque nisi. Aenean pellentesque fermentum ipsum vel pretium. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas.",
		"Curabitur tempus lectus nisl, ut tempus nunc efficitur et. Maecenas vel nulla et ipsum sollicitudin consectetur ac a dolor. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed vulputate a tellus sit amet pellentesque. Cras sapien nunc, efficitur eu mi nec, tincidunt finibus dolor. Nullam volutpat accumsan elit et efficitur. Nullam felis enim, consequat quis semper nec, gravida id dui. Mauris et lorem ac odio eleifend tempor id vitae dolor. Quisque ante leo, vestibulum id leo sed, bibendum venenatis mauris. Fusce bibendum placerat nibh, in posuere magna placerat ac. Duis imperdiet mauris at risus aliquam, in congue lectus hendrerit. Phasellus porttitor metus eu purus rhoncus, ut iaculis dolor condimentum.",
		"Sed eu ante lorem. Donec diam odio, suscipit eu euismod eget, sagittis non dui. Mauris consectetur volutpat viverra. Curabitur lacinia, dui vel aliquam lacinia, felis turpis bibendum odio, non ultricies augue augue nec diam. Etiam posuere risus sodales nisi gravida, eu venenatis turpis fermentum. Nulla vestibulum nec purus ut imperdiet. Vivamus vestibulum elementum nisi non molestie. Integer sit amet dignissim erat, sed lacinia massa. Sed ac nulla leo. Nulla ex purus, consequat at lectus vel, mollis tempor justo.",
		"Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Vivamus convallis nulla facilisis odio viverra, quis malesuada leo viverra. Etiam neque purus, interdum eu dui et, facilisis congue felis. Nunc tempor risus ut sapien porta, non tincidunt magna ornare. Vivamus consectetur, ligula in facilisis consequat, turpis quam fermentum ipsum, eget elementum odio velit vel risus. Etiam maximus eleifend lacus et cursus. Sed feugiat dolor et velit suscipit, vel mattis orci auctor. Nam tempor nunc vitae turpis interdum mollis. Interdum et malesuada fames ac ante ipsum primis in faucibus. Vestibulum sit amet mauris a nulla consequat blandit. Aenean fringilla orci fringilla lectus pulvinar, eu pellentesque erat egestas. Vivamus venenatis, tortor at tincidunt fermentum, metus ligula blandit massa, id congue metus eros sit amet nisi. Etiam ac vehicula ex, sit amet mollis mauris. Suspendisse potenti. Praesent ut eleifend ante.",
	}
)

func randInt(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}

func _createNotes(session *session.Session, input map[string]string) (so SyncOutput, err error) {
	var newNotes Items

	for k, v := range input {
		newNote, _ := NewNote(k, v, nil)
		newNotes = append(newNotes, &newNote)
	}

	if len(newNotes) > 0 {
		eNotes, _ := newNotes.Encrypt(testSession, session.DefaultItemsKey)

		si := SyncInput{
			Session: session,
			Items:   eNotes,
		}

		so, err = Sync(si)
		if err != nil {
			err = fmt.Errorf("PutItems Failed: %v", err)
		}
	}

	return
}

func _createTags(session *session.Session, input []string) (output SyncOutput, err error) {
	for _, tt := range input {
		newTag, _ := NewTag(tt, nil)
		newTagContent := TagContent{
			Title: tt,
		}
		newTagContent.SetUpdateTime(time.Now())
		newTag.Content = newTagContent

		var eItem EncryptedItem

		eItem, err = EncryptItem(&newTag, session.DefaultItemsKey, session)
		if err != nil {
			return
		}

		si := SyncInput{
			Session: session,
			Items:   EncryptedItems{eItem},
		}
		output, err = Sync(si)

		if err != nil {
			err = fmt.Errorf("PutItems Failed: %v", err)

			return
		}
	}

	return
}

func _deleteAllTagsNotesComponents(s *session.Session) (err error) {
	var so SyncOutput

	so, err = Sync(SyncInput{
		Session: s,
	})
	if err != nil {
		return
	}

	var toDel EncryptedItems

	for _, itemsKey := range s.ItemsKeys {
		if itemsKey.UUID != s.DefaultItemsKey.UUID {
			var eik EncryptedItem

			eik, err = EncryptItemsKey(itemsKey, s, false)
			if err != nil {
				return
			}

			eik.Content = ""
			eik.ItemsKeyID = ""
			eik.EncItemKey = ""
			eik.Deleted = true

			toDel = append(toDel, eik)
		}
	}

	for _, item := range so.Items {
		var del bool

		switch item.ContentType {
		case "Note":
			del = true
		case "Tag":
			del = true
		case "SN|Component":
			del = true
		case "SN|UserPreferences":
			del = true

		default:
			continue
		}

		if del {
			item.Deleted = true
			item.EncItemKey = ""
			item.Content = ""
			item.ItemsKeyID = ""
			toDel = append(toDel, item)
		}
	}

	if len(toDel) > 0 {
		_, err = Sync(SyncInput{
			Session: s,
			Items:   toDel,
		})
		if err != nil {
			return fmt.Errorf("PutItems Failed: %v", err)
		}
	}

	return err
}

func _getItems(session *session.Session, itemFilters ItemFilters) (items Items, err error) {
	si := SyncInput{
		Session: session,
	}

	var so SyncOutput

	so, err = Sync(si)
	if err != nil {
		err = fmt.Errorf("sync failed: %v", err)

		return
	}

	items, err = so.Items.DecryptAndParse(session)
	if err != nil {
		return
	}

	items.Filter(itemFilters)

	return
}

func createNote(title, text, uuid string) *Note {
	note, _ := NewNote(title, text, nil)
	if uuid != "" {
		note.UUID = uuid
	}

	return &note
}

func createTag(title, uuid string, refs ItemReferences) (tagp *Tag, err error) {
	tag, err := NewTag(title, refs)
	if err != nil {
		return
	}

	if uuid != "" {
		tag.UUID = uuid
	}

	content := NewTagContent()
	content.Title = title
	content.ItemReferences = refs
	tag.Content = *content

	return &tag, err
}

func cleanup() {
	if err := _deleteAllTagsNotesComponents(testSession); err != nil {
		log.Fatal(err)
	}
}

func tempFilePath() string {
	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		log.Fatal("failed to created temporary directory")
	}

	return filepath.Join(dir, "tmpfile")
}

func TestReEncrypt(t *testing.T) {
	if !strings.Contains(testSession.Server, "ramea") {
		return
	}

	localTestMain()

	defer cleanup()

	_, err := Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	tag, _ := createTag("test", GenUUID(), nil)

	itr, err := EncryptItem(tag, testSession.DefaultItemsKey, testSession)
	require.NoError(t, err)

	nik := NewItemsKey()
	nie, err := ReEncryptItem(itr, testSession.DefaultItemsKey, nik, testSession.MasterKey, testSession)
	require.NoError(t, err)
	require.Equal(t, nie.UUID, itr.UUID)
	require.NotEqual(t, nie.ItemsKeyID, itr.ItemsKeyID)

	testSession.ItemsKeys = append(testSession.ItemsKeys, session.SessionItemsKey{
		UUID:     nik.UUID,
		ItemsKey: nik.ItemsKey,
		Default:  nik.Default,
	})

	ni2, err := DecryptAndParseItem(nie, testSession)
	require.NoError(t, err)

	rt := ni2.(*Tag)
	require.Equal(t, rt.Content.Title, tag.Content.Title)
}

// func TestRegisterExportImport(t *testing.T) {
//	if !strings.Contains(testSession.Server, "ramea") {
//		return
//	}
//
//	localTestMain()
//	defer cleanup()
//
//	tmpfn := tempFilePath()
//
//	require.NoError(t, testSession.Export(tmpfn))
//
//	so, err := Sync(SyncInput{
//		Session: testSession,
//	})
//	require.NoError(t, err)
//
//	items, key, err := testSession.Import(tmpfn, so.SyncToken)
//	require.NoError(t, err)
//	require.Len(t, items, 1)
//	require.Equal(t, common.SNItemTypeItemsKey, items[0].ContentType)
//	require.Equal(t, items[0].UUID, testSession.DefaultItemsKey.UUID)
//	require.Equal(t, testSession.DefaultItemsKey.ItemsKey, key.ItemsKey)
// }

// TODO: Re-enable export import test
// func TestRegisterCreateTagExportImportTwice(t *testing.T) {
// 	if !strings.Contains(testSession.Server, "ramea") {
// 		return
// 	}
//
// 	defer cleanup()
//
// 	tag, _ := createTag("test-title", "", nil)
//
// 	var initItems Items
// 	initItems = append(initItems, tag)
//
// 	encInitItems, err := initItems.Encrypt(testSession, testSession.DefaultItemsKey)
// 	require.NoError(t, err)
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 		Items:   encInitItems,
// 	})
// 	require.NoError(t, err)
// 	initKey := testSession.DefaultItemsKey
// 	require.Equal(t, 1, len(so.SavedItems))
// 	require.Equal(t, "Tag", so.SavedItems[0].ContentType)
//
// 	tmpfn := tempFilePath()
// 	require.NoError(t, testSession.Export(tmpfn))
//
// 	items, key, err := testSession.Import(tmpfn, "", "")
// 	require.NoError(t, err)
// 	require.Len(t, items, 2)
//
// 	var keyIndex int
//
// 	for x := range items {
// 		if items[x].ContentType == common.SNItemTypeItemsKey {
// 			keyIndex = x
// 		}
// 	}
//
// 	require.Equal(t, common.SNItemTypeItemsKey, items[keyIndex].ContentType)
// 	require.Equal(t, initKey.UpdatedAtTimestamp, key.UpdatedAtTimestamp)
//
// 	items, key, err = testSession.Import(tmpfn, "", "")
// 	require.NoError(t, err)
// 	require.Len(t, items, 2)
//
// 	for x := range items {
// 		if items[x].ContentType == common.SNItemTypeItemsKey {
// 			keyIndex = x
// 		}
// 	}
// }

// TODO: Re-enable export import test
// func TestRegisterCreateTagExportImport(t *testing.T) {
// 	if !strings.Contains(testSession.Server, "ramea") {
// 		return
// 	}
//
// 	defer cleanup()
//
// 	tag, _ := createTag("test-title", "", nil)
//
// 	var initItems Items
// 	initItems = append(initItems, tag)
//
// 	encInitItems, err := initItems.Encrypt(testSession, testSession.DefaultItemsKey)
// 	require.NoError(t, err)
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 		Items:   encInitItems,
// 	})
// 	require.NoError(t, err)
// 	initKey := testSession.DefaultItemsKey
// 	require.Equal(t, 1, len(so.SavedItems))
// 	require.Equal(t, "Tag", so.SavedItems[0].ContentType)
//
// 	tmpfn := tempFilePath()
// 	require.NoError(t, testSession.Export(tmpfn))
//
// 	items, key, err := testSession.Import(tmpfn, "", "")
// 	require.NoError(t, err)
//
// 	var keyIndex int
// 	for x := range items {
// 		if items[x].ContentType == common.SNItemTypeItemsKey {
// 			keyIndex = x
// 		}
// 	}
//
// 	require.Equal(t, common.SNItemTypeItemsKey, items[keyIndex].ContentType)
// 	require.Equal(t, initKey.UpdatedAtTimestamp, key.UpdatedAtTimestamp)
// }
//
// func TestRegisterCreateTagExportImportIntoNewAccount(t *testing.T) {
// 	if !strings.Contains(testSession.Server, "ramea") {
// 		return
// 	}
//
// 	defer cleanup()
//
// 	// create a tag and sync it to SN using the test session
// 	tag, _ := createTag("test-title", "", nil)
//
// 	var initItems Items
// 	initItems = append(initItems, tag)
//
// 	encInitItems, err := initItems.Encrypt(testSession, testSession.DefaultItemsKey)
// 	require.NoError(t, err)
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 		Items:   encInitItems,
// 	})
// 	require.NoError(t, err)
//
// 	initKey := testSession.DefaultItemsKey
//
// 	require.Equal(t, 1, len(so.SavedItems))
// 	require.Equal(t, "Tag", so.SavedItems[0].ContentType)
//
// 	tmpfn := tempFilePath()
// 	require.NoError(t, testSession.Export(tmpfn))
//
// 	// now register and sign in as a new user
// 	secondUserEmail := fmt.Sprintf("ramea-%s", strconv.FormatInt(time.Now().UnixNano(), 16))
// 	secondUserPassword := "secretsanta2"
//
// 	ri := RegisterInput{
// 		Password:  secondUserPassword,
// 		Email:     secondUserEmail,
// 		APIServer: "http://ramea:3000",
// 		Debug:     true,
// 	}
//
// 	_, err = ri.Register()
// 	require.NoError(t, err)
// 	o, err := SignIn(SignInInput{
// 		Email:     secondUserEmail,
// 		Password:  "secretsanta2",
// 		APIServer: "http://ramea:3000",
// 		Debug:     true,
// 	})
// 	require.NoError(t, err)
//
// 	newSession := o.Session
//
// 	// importing with wrong password should fail
// 	_, _, err = newSession.Import(tmpfn, "", "secretsanta1")
// 	require.Error(t, err)
//
// 	// importing with correct password should succeed
// 	items, key, err := newSession.Import(tmpfn, "", "secretsanta")
// 	require.NoError(t, err)
//
// 	var keyIndex int
//
// 	for x := range items {
// 		if items[x].ContentType == common.SNItemTypeItemsKey {
// 			keyIndex = x
// 		}
// 	}
//
// 	require.Equal(t, common.SNItemTypeItemsKey, items[keyIndex].ContentType)
// 	require.NotEqual(t, initKey.UpdatedAtTimestamp, key.UpdatedAtTimestamp)
//
// 	// now sync and decrypt
// 	so, err = Sync(SyncInput{
// 		Session: &newSession,
// 	})
//
// 	require.NoError(t, err)
//
// 	dis, err := so.Items.DecryptAndParse(&newSession)
// 	require.NoError(t, err)
//
// 	var found bool
// 	for x := range dis {
// 		if dis[x].GetContentType() == "Tag" {
// 			nt := dis[x].(*Tag)
// 			require.Equal(t, nt.Content.Title, tag.Content.Title)
// 			// these won't be equal as we're importing an item with the same UUID as
// 			// the original tag, resulting in a uuid_conflict, so we generate a new one
// 			require.NotEqual(t, nt.UUID, tag.UUID)
//
// 			found = true
//
// 			break
// 		}
// 	}
//
// 	require.True(t, found)
// }

// 1. add a tag to SN (encrypted with initial items key) - KEY1
// 2. take export (generate new items key, encrypt tag and new items key, and write to file only)
// 3. add another tag to SN (encrypted with initial items key) - KEY1
// 4. import - KEY 2
// the resulting state should be:
// 2 tags in SN, both encrypted with items key in export
// 1 items key in session and SN

// func TestRegisterCreateTagExportCreateTagImport(t *testing.T) {
// 	if !strings.Contains(testSession.Server, "ramea") {
// 		return
// 	}
//
// 	defer cleanup()
//
// 	tag, _ := createTag("test-title", "", nil)
// 	var initItems Items
// 	initItems = append(initItems, tag)
//
// 	encInitItems, _ := initItems.Encrypt(testSession, testSession.DefaultItemsKey)
// 	so, _ := Sync(SyncInput{
// 		Session: testSession,
// 		Items:   encInitItems,
// 	})
// 	initKey := testSession.DefaultItemsKey
// 	require.Equal(t, 1, len(so.SavedItems))
// 	require.Equal(t, "Tag", so.SavedItems[0].ContentType)
//
// 	tmpfn := tempFilePath()
// 	require.NoError(t, testSession.Export(tmpfn))
// 	tag2, _ := createTag("another-test-title", "", nil)
// 	var items2 Items
// 	items2 = append(items2, tag2)
// 	encItems2, _ := items2.Encrypt(testSession, testSession.DefaultItemsKey)
//
// 	so2, _ := Sync(SyncInput{
// 		Session:   testSession,
// 		Items:     encItems2,
// 		SyncToken: so.SyncToken,
// 	})
//
// 	require.Equal(t, 1, len(so2.SavedItems))
// 	require.Equal(t, 0, len(so2.Items))
//
// 	items, key, err := testSession.Import(tmpfn, so.SyncToken, "")
// 	require.NoError(t, err)
//
// 	require.Len(t, testSession.ItemsKeys, 1)
// 	require.Equal(t, testSession.ItemsKeys[0].UUID, testSession.DefaultItemsKey.UUID)
//
// 	require.NoError(t, err)
// 	require.Len(t, items, 3)
//
// 	// check the new items key differs from initial (session should now have items key generated by export)
// 	require.NotEqual(t, testSession.DefaultItemsKey.ItemsKey, initKey.ItemsKey)
// 	// check we now have two tags encrypted by that new key that's loaded in the session
// 	var finalEncTags EncryptedItems
//
// 	for x := range items {
// 		if items[x].ContentType == "Tag" {
// 			finalEncTags = append(finalEncTags, items[x])
// 		}
// 	}
//
// 	finalTags, err := finalEncTags.DecryptAndParse(testSession)
// 	require.NoError(t, err)
// 	require.Len(t, finalTags, 2)
//
// 	// the key has been re-generated but meta-data such as UUID and times should be the same
// 	require.NotEqual(t, initKey.ItemsKey, key.ItemsKey)
// 	require.Equal(t, initKey.UUID, key.UUID)
// 	require.Equal(t, initKey.UpdatedAtTimestamp, key.UpdatedAtTimestamp)
// }

func TestNewNoteContent(t *testing.T) {
	defer cleanup()

	note, err := NewNote("test-title", "test-text", nil)
	require.NoError(t, err, "NewNote Failed")
	nc, err := json.Marshal(note.Content)
	// nc, err := json.MarshalIndent(note.Content, "", "  ")
	require.NoError(t, err, "Marshal Failed")
	var v interface{}
	err = json.Unmarshal(nc, &v)
	require.NoError(t, err, "Unmarshal Failed")
	if testSession.Schemas[noteContentSchemaName] == nil {
		err = fmt.Errorf("Schema %s not found", noteContentSchemaName)
		require.NoError(t, err, "Schema Not Found")
	}
	err = validateContentSchema(testSession.Schemas[noteContentSchemaName], v)
	require.NoError(t, err, "validateContentSchema Failed")

	require.Equal(t, "test-text", note.Content.GetText())
}

func TestAddDeleteNote(t *testing.T) {
	defer cleanup()

	randPara := "TestText"
	newNote, _ := NewNote("TestTitle", randPara, nil)
	dItems := Items{&newNote}
	require.NoError(t, dItems.Validate(testSession))
	eItems, err := dItems.Encrypt(testSession, testSession.DefaultItemsKey)
	// fmt.Printf("NEW ?NOTE: %#+v\n", eItems)
	var foundItemsKeyInList bool

	for x := range testSession.ItemsKeys {
		if testSession.ItemsKeys[x].UUID == testSession.DefaultItemsKey.UUID {
			foundItemsKeyInList = true
		}
	}

	require.True(t, foundItemsKeyInList)
	require.NoError(t, err)
	require.NotEmpty(t, eItems)

	testSession.Debug = true
	si := SyncInput{
		Items:   eItems,
		Session: testSession,
	}

	var so SyncOutput
	so, err = Sync(si)
	// fmt.Println("ITEMS")
	// for _, f := range so.Items {
	// 	fmt.Printf("%#+v\n", f)
	// }
	// fmt.Println("SAVED")
	// for _, f := range so.SavedItems {
	// 	fmt.Printf("%#+v\n", f)
	// }
	// fmt.Println("DONE")
	require.NoError(t, err, "Sync Failed")
	require.Len(t, so.SavedItems, 1, "expected one")
	require.Len(t, so.Conflicts, 0, "expected none")
	uuidOfNewItem := so.SavedItems[0].UUID

	var foundItem bool

	for i := range so.SavedItems {
		if so.SavedItems[i].UUID == uuidOfNewItem {
			foundItem = true
			// foundItem = so.Items[i]
			//
			// ni := items[i].(*Note)
			//
			// if ni.ContentType != "Note" {
			// 	t.Errorf("content type of new item is incorrect - expected: Note got: %s",
			// 		items[i].GetContentType())
			// }
			//
			// if ni.Deleted {
			// 	t.Errorf("deleted status of new item is incorrect - expected: False got: True")
			// }
			//
			// if ni.Content.GetText() != randPara {
			// 	t.Errorf("text of new item is incorrect - expected: %s got: %s",
			// 		randPara, ni.Content.GetText())
			// }
		}
	}

	// if foundItem.GetUUID() == "" {
	if !foundItem {
		t.Errorf("failed to get created Item by UUID")
	}

	// is := Items{foundItem}
	// eis, err := is.Encrypt(testSession, testSession.DefaultItemsKey)
	// require.Len(t, eis, 1)
	// itemToDelete := eis[0]
	// itemToDelete.Deleted = true
	// itemToDelete.Content = ""
	// itemToDelete.EncItemKey = ""
	// itemToDelete.ItemsKeyID = ""
	// eis = EncryptedItems{
	// 	itemToDelete,
	// }
	//
	// require.NoError(t, err)
	//
	// st := so.SyncToken
	// so, err = Sync(SyncInput{
	// 	Session:   testSession,
	// 	SyncToken: st,
	// 	Items:     eis,
	// })
	// require.NoError(t, err, "Sync to Delete Failed", err)
	// require.Len(t, so.SavedItems, 1)
}

func TestCreateItemsKey(t *testing.T) {
	ik, err := CreateItemsKey()
	require.NoError(t, err)
	require.Equal(t, common.SNItemTypeItemsKey, ik.ContentType)
	require.False(t, ik.Deleted)
	require.NotEmpty(t, ik.UUID)
	require.NotEmpty(t, ik.Content)
	require.NotEmpty(t, ik.CreatedAt)
	require.NotEmpty(t, ik.CreatedAtTimestamp)
	require.NotEmpty(t, ik.ItemsKey)
	require.Empty(t, ik.UpdatedAtTimestamp)
	require.Empty(t, ik.UpdatedAt)
}

func TestEncryptDecryptItemWithItemsKey(t *testing.T) {
	ik, err := CreateItemsKey()
	require.NoError(t, err)
	require.Equal(t, common.SNItemTypeItemsKey, ik.ContentType)
	require.False(t, ik.Deleted)
	require.NotEmpty(t, ik.UUID)
	require.NotEmpty(t, ik.Content)
	require.NotEmpty(t, ik.CreatedAt)
	require.NotEmpty(t, ik.CreatedAtTimestamp)
	require.Empty(t, ik.UpdatedAtTimestamp)
	require.Empty(t, ik.UpdatedAt)

	sik := session.SessionItemsKey{
		UUID:     ik.UUID,
		ItemsKey: ik.ItemsKey,
		Default:  ik.Default,
	}

	n, _ := NewNote("test title", "test content", nil)
	ei, err := EncryptItem(&n, sik, testSession)
	require.NoError(t, err)
	require.NotEmpty(t, ei.UUID)
	require.NotEmpty(t, ei.CreatedAtTimestamp)
	require.NotEmpty(t, ei.CreatedAt)
	require.False(t, ei.Deleted)
	require.Equal(t, "Note", ei.ContentType)
	require.NotEmpty(t, ei.Content)
	require.Empty(t, ei.DuplicateOf)

	// copy the struct so we don't pollute existing
	duplicateSession := session.Session{
		Debug:             testSession.Debug,
		Server:            testSession.Server,
		Token:             testSession.Token,
		MasterKey:         testSession.MasterKey,
		ItemsKeys:         testSession.ItemsKeys,
		DefaultItemsKey:   testSession.DefaultItemsKey,
		KeyParams:         testSession.KeyParams,
		AccessToken:       testSession.AccessToken,
		RefreshToken:      testSession.RefreshToken,
		AccessExpiration:  testSession.AccessExpiration,
		RefreshExpiration: testSession.RefreshExpiration,
		PasswordNonce:     testSession.PasswordNonce,
	}

	duplicateSession.DefaultItemsKey = sik
	duplicateSession.ItemsKeys = append(duplicateSession.ItemsKeys, sik)

	e := EncryptedItems{ei}
	di, err := DecryptItems(&duplicateSession, e, []session.SessionItemsKey{})

	require.NoError(t, err)
	require.NotEmpty(t, di)

	var dn Note

	for _, dItem := range di {
		if dItem.ContentType == "Note" {
			dn = *parseNote(dItem).(*Note)
			break
		}
	}

	require.NotEmpty(t, dn.Content)
	require.Equal(t, "Note", dn.ContentType)
	require.Equal(t, "test title", dn.Content.GetTitle())
	require.Equal(t, "test content", dn.Content.GetText())
	require.NotEmpty(t, dn.UUID)
	require.NotEmpty(t, dn.CreatedAtTimestamp)
	require.NotEmpty(t, dn.CreatedAt)
	require.False(t, dn.Deleted)
	// TODO: should have DuplicateOf?
	// require.Empty(t, dn.du)
}

func TestProcessContentModel(t *testing.T) {
	output, err := processContentModel("Note", `
    {
        "title": "Todo1",
        "text": "{\n  \"schemaVersion\": \"1.0.0\",\n  \"groups\": [\n    {\n      \"name\": \"ddd\",\n      \"tasks\": [\n        {\n          \"id\": \"23e56588-ba63-494f-89fe-44f556682fe3\",\n          \"description\": \"ddd-first\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-10T18:39:19.471Z\",\n          \"updatedAt\": \"2023-12-10T18:39:26.781Z\"\n        }\n      ],\n      \"lastActive\": \"2023-12-10T18:40:06.294Z\",\n      \"sections\": [\n        {\n          \"id\": \"open-tasks\",\n          \"name\": \"Open\",\n          \"collapsed\": false\n        },\n        {\n          \"id\": \"completed-tasks\",\n          \"name\": \"Completed\",\n          \"collapsed\": false\n        }\n      ],\n      \"collapsed\": false\n    },\n    {\n      \"name\": \"homelab\",\n      \"tasks\": [\n        {\n          \"id\": \"a2acb45d-14b6-4e0b-b24c-88596d7d10f4\",\n          \"description\": \"homelab-2\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-10T18:40:45.555Z\"\n        },\n        {\n          \"id\": \"2ff0639d-2f72-4cf5-a1df-e207e97ffe0b\",\n          \"description\": \"homelab-1\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-10T18:40:41.969Z\"\n        }\n      ],\n      \"lastActive\": \"2023-12-10T18:40:45.555Z\",\n      \"sections\": [\n        {\n          \"id\": \"open-tasks\",\n          \"name\": \"Open\",\n          \"collapsed\": false\n        },\n        {\n          \"id\": \"completed-tasks\",\n          \"name\": \"Completed\",\n          \"collapsed\": false\n        }\n      ],\n      \"collapsed\": false\n    }\n  ],\n  \"defaultSections\": [\n    {\n      \"id\": \"open-tasks\",\n      \"name\": \"Open\"\n    },\n    {\n      \"id\": \"completed-tasks\",\n      \"name\": \"Completed\"\n    }\n  ]\n}",
        "references": [],
        "appData": {
          "org.standardnotes.sn": {
            "client_updated_at": "2023-12-10T18:40:45.808Z",
            "prefersPlainEditor": false,
            "pinned": false
          },
          "org.standardnotes.sn.components": {
            "f90fc73b-b53b-485c-beb2-ac7f1709d22a": {}
          }
        },
        "preview_plain": "0/3 tasks completed",
        "spellcheck": true,
        "preview_html": "\\u003cdiv class=\"flex flex-grow items-center mb-3\"\\u003e\\u003csvg data-testid=\"circular-progress-bar\" class=\"sk-circular-progress\" viewBox=\"0 0 18 18\"\\u003e\\u003ccircle class=\"background\"\\u003e\\u003c/circle\\u003e\\u003ccircle class=\"progress p-0\"\\u003e\\u003c/circle\\u003e\\u003c/svg\\u003e\\u003cp class=\"ml-2 w-full font-medium\"\\u003e0\\u003c!-- --\\u003e/\\u003c!-- --\\u003e3\\u003c!-- --\\u003e tasks completed\\u003c/p\\u003e\\u003c/div\\u003e\\u003cdiv class=\"my-2\"\\u003e\\u003cp data-testid=\"group-summary\" class=\"mb-1\"\\u003eddd\\u003cspan class=\"px-2 neutral\"\\u003e0\\u003c!-- --\\u003e/\\u003c!-- --\\u003e1\\u003c/span\\u003e\\u003c/p\\u003e\\u003cp data-testid=\"group-summary\" class=\"mb-1\"\\u003ehomelab\\u003cspan class=\"px-2 neutral\"\\u003e0\\u003c!-- --\\u003e/\\u003c!-- --\\u003e2\\u003c/span\\u003e\\u003c/p\\u003e\\u003c/div\\u003e",
        "noteType": "task",
        "editorIdentifier": "com.sncommunity.advanced-checklist"
    }`)
	require.NoError(t, err)
	require.NotNil(t, output)
	noteContent := output.(*NoteContent)
	require.Equal(t, "Todo1", noteContent.Title)
	require.Empty(t, noteContent.References())
	require.Equal(t, "com.sncommunity.advanced-checklist", noteContent.EditorIdentifier)
	require.Equal(t, "task", noteContent.NoteType)
	require.Equal(t, "0/3 tasks completed", noteContent.PreviewPlain)
	require.True(t, noteContent.Spellcheck)
}

//
// func TestImportFromFile(t *testing.T) {
// 	cleanup()
// 	defer cleanup()
//
// 	numItemsKeys := len(testSession.ItemsKeys)
// 	itemsKeyPre := testSession.DefaultItemsKey.ItemsKey
// 	require.NotEmpty(t, testSession.DefaultItemsKey.ItemsKey)
//
// 	_, itemsKey, err := testSession.Import("testuser-encrypted-backup.txt", "", "testuser")
// 	require.NoError(t, err)
// 	require.NotEmpty(t, itemsKey)
//
// 	require.Len(t, testSession.ItemsKeys, numItemsKeys)
// 	require.NotEmpty(t, testSession.DefaultItemsKey.ItemsKey)
// 	require.NotEqual(t, itemsKeyPre, testSession.DefaultItemsKey.ItemsKey)
// 	require.Equal(t, itemsKey.ItemsKey, testSession.ItemsKeys[0].ItemsKey)
//
// 	// fresh sync and check items exist
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 	})
// 	require.NoError(t, err)
// 	dis, err := so.Items.DecryptAndParse(testSession)
// 	require.NoError(t, err)
// 	var noteFound bool
// 	for _, d := range dis {
// 		if d.GetContentType() == "Note" {
// 			noteFound = true
// 		}
// 	}
//
// 	require.True(t, noteFound)
// }
//
// func TestExportImportOfNote(t *testing.T) {
// 	cleanup()
// 	defer cleanup()
//
// 	n, _ := NewNote("test title", "test content", nil)
// 	path := "./test.json"
//
// 	// sync new note to SN before export
// 	ei, err := EncryptItem(&n, testSession.DefaultItemsKey, testSession)
// 	require.NoError(t, err)
//
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 		Items:   EncryptedItems{ei},
// 	})
//
// 	require.NoError(t, err)
// 	require.NotEmpty(t, so.SavedItems)
// 	require.Equal(t, "Note", so.SavedItems[0].ContentType)
// 	eitd := append(so.SavedItems, so.Items...)
// 	itemsToExport, err := eitd.DecryptAndParse(testSession)
// 	require.NoError(t, err)
// 	require.GreaterOrEqual(t, len(itemsToExport), 1)
// 	require.NoError(t, testSession.Export(path))
//
// 	importedItems, _, err := testSession.Import(path, so.SyncToken, "")
// 	require.NoError(t, err)
// 	require.NotEmpty(t, importedItems)
//
// 	var foundNote bool
//
// 	for x := range importedItems {
// 		if importedItems[x].ContentType == "Note" {
// 			var i Item
// 			i, err = DecryptAndParseItem(importedItems[x], testSession)
//
// 			require.NoError(t, err)
//
// 			in := i.(*Note)
//
// 			require.Equal(t, n.Content.Title, in.Content.Title)
// 			require.Equal(t, n.Content.Text, in.Content.Text)
//
// 			foundNote = true
//
// 			break
// 		}
// 	}
//
// 	require.True(t, foundNote)
// }

// func TestExportImportOfNoteWithSync(t *testing.T) {
//	cleanup()
//	defer cleanup()
//	syncOutput, err := Sync(SyncInput{
//		Session: testSession,
//	})
//	require.NoError(t, err)
//
//	n := NewNote()
//	nc := NewNoteContent()
//	nc.Title = "test title"
//	nc.Text = "test content"
//	n.Content = *nc
//
//	path := "./test.json"
//
//	require.NoError(t, Items{&n}.Export(testSession, path))
//
//	importedItems, itemsKey, err := testSession.Import(path, true, syncOutput.SyncToken)
//	require.NoError(t, err)
//	require.NotEmpty(t, importedItems)
//	require.Equal(t, itemsKey.UUID, testSession.DefaultItemsKey.UUID)
//	require.Equal(t, itemsKey.ItemsKey, testSession.DefaultItemsKey.ItemsKey)
//
//	var foundNote bool
//
//	var importedNote *Note
//
//	for x := range importedItems {
//		if importedItems[x].GetContentType() == "Note" {
//			in := importedItems[x].(*Note)
//			require.Equal(t, n.Content.Title, in.Content.Title)
//			require.Equal(t, n.Content.Text, in.Content.Text)
//			// n = *parseNote(importedItems[x]).(*Note)
//			foundNote = true
//			importedNote = in
//
//			break
//		}
//	}
//
//	require.True(t, foundNote)
//
//	// check items key is in session
//	var foundItemsKeyInSession bool
//
//	for _, ik := range testSession.ItemsKeys {
//		if importedNote.ItemsKeyID == ik.UUID {
//			foundItemsKeyInSession = true
//			break
//		}
//	}
//
//	require.True(t, foundItemsKeyInSession)
//
//	syncOutput, err = Sync(SyncInput{
//		Session:   testSession,
//		SyncToken: "",
//	})
//	require.NoError(t, err)
//
//	var foundItemsKeyInSN bool
//
//	for x := range syncOutput.Items {
//		if syncOutput.Items[x].UUID == importedNote.ItemsKeyID {
//			foundItemsKeyInSN = true
//		}
//	}
//
//	require.True(t, foundItemsKeyInSN)
// }

// This test is to prove we always replace any existing keys with matching ones from the export we import.
// func TestDecryptionOfImportedItemsKey(t *testing.T) {
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 	})
//
// 	n, _ := NewNote("test title", "test content", nil)
// 	path := "./test.json"
//
// 	var preExportItemsKeys []ItemsKey
// 	preExportItemsKeys = append(preExportItemsKeys, testSession.ItemsKeys...)
// 	preExportDefaultItemsKey := testSession.DefaultItemsKey
//
// 	items := Items{&n}
//
// 	eis, err := items.Encrypt(testSession, testSession.DefaultItemsKey)
// 	require.NoError(t, err)
//
// 	_, err = Sync(SyncInput{
// 		Session:   testSession,
// 		SyncToken: so.SyncToken,
// 		Items:     eis,
// 	})
// 	require.NoError(t, err)
//
// 	require.NoError(t, testSession.Export(path))
//
// 	// reset the session
// 	testSession.DefaultItemsKey = preExportDefaultItemsKey
// 	testSession.ItemsKeys = preExportItemsKeys
//
// 	importedItems, _, err := testSession.Import(path, "", "")
// 	require.NoError(t, err)
// 	require.NotEmpty(t, importedItems)
//
// 	var foundNote bool
//
// 	for x := range importedItems {
// 		if importedItems[x].ContentType == "Note" {
// 			var i Item
// 			i, err = DecryptAndParseItem(importedItems[x], testSession)
//
// 			require.NoError(t, err)
//
// 			in := i.(*Note)
//
// 			require.Equal(t, n.Content.Title, in.Content.Title)
// 			require.Equal(t, n.Content.Text, in.Content.Text)
//
// 			foundNote = true
//
// 			break
// 		}
// 	}
//
// 	require.True(t, foundNote)
// }

//
// func TestExportImportOfEncryptedNote(t *testing.T) {
//	n := NewNote()
//	nc := NewNoteContent()
//	nc.Title = "test title"
//	nc.Text = "test content"
//	n.Content = *nc
//
//	path := "./test.json"
//
//	require.NoError(t, Items{&n}.Export(testSession, path, false))
//
//	importedItems, itemsKey, err := testSession.Import(path)
//	require.NoError(t, err)
//	require.NotEmpty(t, importedItems)
//	require.Equal(t, itemsKey.UUID, testSession.DefaultItemsKey.UUID)
//	require.Equal(t, itemsKey.ItemsKey, testSession.DefaultItemsKey.ItemsKey)
//
//	var foundNote bool
//
//	for x := range importedItems {
//		if importedItems[x].GetContentType() == "Note" {
//			in := importedItems[x].(*Note)
//			require.Equal(t, n.Content.Title, in.Content.Title)
//			require.Equal(t, n.Content.Text, in.Content.Text)
//			//n = *parseNote(importedItems[x]).(*Note)
//			foundNote = true
//
//			break
//		}
//		//
//	}
//
//	require.True(t, foundNote)
// }
//
// func TestEncryptDecryptItemWithItemsKeyWithExportedMethods(t *testing.T) {
// 	testSession.Debug = true
// 	ik, err := testSession.CreateItemsKey()
// 	require.NoError(t, err)
// 	require.Equal(t, common.SNItemTypeItemsKey, ik.ContentType)
// 	require.False(t, ik.Deleted)
// 	require.NotEmpty(t, ik.UUID)
// 	require.NotEmpty(t, ik.Content)
// 	require.NotEmpty(t, ik.CreatedAt)
// 	require.NotEmpty(t, ik.CreatedAtTimestamp)
// 	require.Empty(t, ik.UpdatedAtTimestamp)
// 	require.Empty(t, ik.UpdatedAt)
//
// 	n, _ := NewNote("test title", "test content", nil)
// 	eis := Items{&n}
// 	encItems, err := eis.Encrypt(testSession, ik)
// 	require.NoError(t, err)
// 	require.NotEmpty(t, encItems[0].UUID)
// 	require.NotEmpty(t, encItems[0].CreatedAtTimestamp)
// 	require.NotEmpty(t, encItems[0].CreatedAt)
// 	require.False(t, encItems[0].Deleted)
// 	require.Equal(t, "Note", encItems[0].ContentType)
// 	require.NotEmpty(t, encItems[0].Content)
// 	require.Empty(t, encItems[0].DuplicateOf)
//
// 	testSession.ItemsKeys = append(testSession.ItemsKeys, ik)
//
// 	di, err := DecryptItems(testSession, encItems, ItemsKeys{})
// 	require.NoError(t, err)
// 	require.NotEmpty(t, di)
//
// 	var dn Note
//
// 	for _, dItem := range di {
// 		if dItem.ContentType == "Note" {
// 			dn = *parseNote(dItem).(*Note)
// 			break
// 		}
// 	}
//
// 	require.NotEmpty(t, dn.Content)
// 	require.Equal(t, "Note", dn.ContentType)
// 	require.Equal(t, "test title", dn.Content.GetTitle())
// 	require.Equal(t, "test content", dn.Content.GetText())
// 	require.NotEmpty(t, dn.UUID)
// 	require.NotEmpty(t, dn.CreatedAtTimestamp)
// 	require.NotEmpty(t, dn.CreatedAt)
// 	require.False(t, dn.Deleted)
// }

func TestCreateAddUseItemsKey(t *testing.T) {
	ik, err := CreateItemsKey()
	require.NoError(t, err)
	require.NotEmpty(t, ik.UUID)
	// require.NotEmpty(t, ik.Version)
	require.NotEmpty(t, ik.ItemsKey)

	randPara := "TestText"
	newNote, _ := NewNote("TestTitle", randPara, nil)
	dItems := Items{&newNote}
	require.NoError(t, dItems.Validate(testSession))
	eItems, err := dItems.Encrypt(testSession, testSession.DefaultItemsKey)

	require.NoError(t, err)
	require.NotEmpty(t, eItems)

	si := SyncInput{
		Items:   eItems,
		Session: testSession,
	}

	var so SyncOutput
	so, err = Sync(si)
	require.NoError(t, err, "Sync Failed", err)
	require.Len(t, so.SavedItems, 1, "expected 1")
	uuidOfNewItem := so.SavedItems[0].UUID
	si = SyncInput{
		Session: testSession,
	}

	so, err = Sync(si)
	require.NoError(t, err, "Sync Failed", err)

	items, err := so.Items.DecryptAndParse(testSession)
	if err != nil {
		return
	}

	var foundCreatedItem bool

	for i := range items {
		if items[i].GetUUID() == uuidOfNewItem {
			foundCreatedItem = true

			ni := items[i].(*Note)

			if ni.ContentType != "Note" {
				t.Errorf("content type of new item is incorrect - expected: Note got: %s",
					items[i].GetContentType())
			}

			if ni.Deleted {
				t.Errorf("deleted status of new item is incorrect - expected: False got: True")
			}

			if ni.Content.GetText() != randPara {
				t.Errorf("text of new item is incorrect - expected: %s got: %s",
					randPara, ni.Content.GetText())
			}
		}
	}

	if !foundCreatedItem {
		t.Errorf("failed to get created Item by UUID")
	}
}

func TestDecryptItemsKeys(t *testing.T) {
	s := testSession

	cleanup()

	defer func() {
		cleanup()
	}()

	syncInput := SyncInput{
		Session: s,
	}

	_, err := Sync(syncInput)
	require.NoError(t, err, "Sync Failed", err)
	require.NotEmpty(t, s.ItemsKeys)
	require.NotEmpty(t, s.DefaultItemsKey)
}

func TestEncryptDecryptItem(t *testing.T) {
	randPara := testParas[randInt(0, len(testParas))]
	newNote, _ := NewNote("TestTitle", randPara, nil)
	dItems := Items{&newNote}
	require.NoError(t, dItems.Validate(testSession))

	// eItems, err := dItems.Encrypt(*testSession)
	eItems, err := dItems.Encrypt(testSession, testSession.DefaultItemsKey)
	require.NoError(t, err)
	require.NotEmpty(t, eItems)

	// Now Decrypt Item
	var items Items
	items, err = eItems.DecryptAndParse(testSession)
	require.NoError(t, err)
	require.NotEmpty(t, items)
}

func TestPutItemsAddSingleNote(t *testing.T) {
	defer cleanup()

	randPara := "TestText"
	newNote, _ := NewNote("TestTitle", randPara, nil)
	dItems := Items{&newNote}
	require.NoError(t, dItems.Validate(testSession))
	eItems, err := dItems.Encrypt(testSession, testSession.DefaultItemsKey)
	require.NoError(t, err)
	require.NotEmpty(t, eItems)

	// fmt.Printf("BEFORE SYNC: %#+v\n", eItems[0])
	si := SyncInput{
		Items:   eItems,
		Session: testSession,
	}

	var so SyncOutput
	so, err = Sync(si)
	require.NoError(t, err, "Sync Failed", err)
	require.Len(t, so.SavedItems, 1, "expected 1")
	uuidOfNewItem := so.SavedItems[0].UUID
	items, err := so.SavedItems.DecryptAndParse(testSession)

	require.NoError(t, err)
	require.NotEmpty(t, items)

	var foundCreatedItem bool

	for i := range items {
		if items[i].GetUUID() == uuidOfNewItem {
			foundCreatedItem = true

			ni := items[i].(*Note)

			if ni.ContentType != "Note" {
				t.Errorf("content type of new item is incorrect - expected: Note got: %s",
					items[i].GetContentType())
			}

			if ni.Deleted {
				t.Errorf("deleted status of new item is incorrect - expected: False got: True")
			}

			if ni.Content.GetText() != randPara {
				t.Errorf("text of new item is incorrect - expected: %s got: %s",
					randPara, ni.Content.GetText())
			}
		}
	}

	if !foundCreatedItem {
		t.Errorf("failed to get created Item by UUID")
	}
}

func TestPutItemsAddSingleComponent(t *testing.T) {
	defer cleanup()

	newComponentContent := ComponentContent{
		Name:               "Minimal Markdown Editor",
		Area:               "editor-editor",
		LocalURL:           "sn://Extensions/org.standardnotes.plus-editor/index.html",
		HostedURL:          "https://extensions.standardnotes.org/e6d4d59ac829ed7ec24e2c139e7d8b21b625dff2d7f98bb7b907291242d31fcd/components/plus-editor",
		OfflineOnly:        false,
		ValidUntil:         "2023-08-29T12:15:17.000Z",
		AutoUpdateDisabled: "",
		DissociatedItemIds: []string{"e9d4daf5-52e6-4d67-975e-a1620bf5217c"},
		AssociatedItemIds:  []string{"d7d1dee3-42f6-3d27-871e-d2320bf3214a"},
		ItemReferences:     nil,
		Active:             true,
		AppData:            AppDataContent{},
	}

	newComponentContent.SetUpdateTime(time.Now())

	newComponent := NewComponent()
	newComponent.Content = newComponentContent

	newComponent.Content.DisassociateItems([]string{"d7d1dee3-42f6-3d27-871e-d2320bf3214a"})
	require.NotContains(t, newComponent.Content.GetItemAssociations(), "d7d1dee3-42f6-3d27-871e-d2320bf3214a")

	newComponent.Content.AssociateItems([]string{"d7d1dee3-42f6-3d27-871e-d2320bf3214a"})
	require.Contains(t, newComponent.Content.GetItemAssociations(), "d7d1dee3-42f6-3d27-871e-d2320bf3214a")

	dItems := Items{&newComponent}
	require.NoError(t, dItems.Validate(testSession))

	eItems, err := dItems.Encrypt(testSession, testSession.DefaultItemsKey)
	require.NoError(t, err)

	syncInput := SyncInput{
		Items:   eItems,
		Session: testSession,
	}

	syncOutput, err := Sync(syncInput)
	require.NoError(t, err, "PutItems Failed", err)
	require.Len(t, syncOutput.SavedItems, 1, "expected 1")
	require.Equal(t, syncInput.Items[0].UUID, syncOutput.SavedItems[0].UUID, "expected UUIDs to be equal")
	uuidOfNewItem := syncOutput.SavedItems[0].UUID

	var items Items
	items, err = syncOutput.SavedItems.DecryptAndParse(testSession)
	require.NoError(t, err)
	require.NotEmpty(t, items)

	var foundCreatedItem bool

	for i := range items {
		if items[i].GetUUID() == uuidOfNewItem {
			foundCreatedItem = true

			require.Equal(t, "SN|Component", items[i].GetContentType())
			require.Equal(t, false, items[i].IsDeleted())
			require.Equal(t, "Minimal Markdown Editor", items[i].(*Component).Content.GetName())
		}
	}

	require.True(t, foundCreatedItem, "failed to get created Item by UUID")
}

func TestItemsRemoveDeleted(t *testing.T) {
	noteOne, _ := NewNote("Title", "Text", nil)
	noteTwo := noteOne.Copy()
	noteTwo.UUID += "a"
	noteThree := noteOne.Copy()
	noteThree.UUID += "b"

	require.False(t, noteOne.Deleted)

	require.False(t, noteTwo.Deleted)

	require.False(t, noteThree.Deleted)

	noteTwo.Deleted = true
	notes := Notes{noteOne, noteTwo, noteThree}
	require.Len(t, notes, 3)
	notes.RemoveDeleted()
	require.Len(t, notes, 2)

	for _, n := range notes {
		require.NotEqual(t, n.UUID, noteTwo.UUID)
	}
}

func TestDecryptedItemsRemoveDeleted(t *testing.T) {
	diOne := DecryptedItem{
		UUID:        "1234",
		Content:     "abcd",
		ContentType: "Note",
		Deleted:     false,
	}
	diTwo := DecryptedItem{
		UUID:        "2345",
		Content:     "abcd",
		ContentType: "Note",
		Deleted:     true,
	}
	diThree := DecryptedItem{
		UUID:        "3456",
		Content:     "abcd",
		ContentType: "Note",
		Deleted:     false,
	}
	dis := DecryptedItems{diOne, diTwo, diThree}
	require.Len(t, dis, 3)
	dis.RemoveDeleted()
	require.Len(t, dis, 2)

	for _, n := range dis {
		require.NotEqual(t, n.UUID, diTwo.UUID)
	}
}

func TestNoteContentCopy(t *testing.T) {
	initialNoteTitle := "Title"
	initialNoteText := "Title"
	initialNoteContent := NewNoteContent()
	initialNoteContent.Title = initialNoteTitle
	initialNoteContent.Text = initialNoteText
	dupeNoteContent := initialNoteContent.Copy()
	// update initial to ensure copy
	initialNoteContent.Title = "Updated Title"
	initialNoteContent.Text = "Updated Text"
	// now check duplicate is correct
	require.NotNil(t, dupeNoteContent)
	require.Equal(t, initialNoteTitle, dupeNoteContent.Title)
	require.Equal(t, initialNoteText, dupeNoteContent.GetText())
}

func TestTagContentCopy(t *testing.T) {
	initialTagTitle := "Title"
	initialTagContent := NewTagContent()
	initialTagContent.Title = initialTagTitle
	dupeNoteContent := initialTagContent.Copy()
	// update initial to ensure copy
	initialTagContent.Title = "Updated Title"
	// now check duplicate is correct
	require.NotNil(t, dupeNoteContent)
	require.Equal(t, initialTagTitle, dupeNoteContent.Title)
}

func TestNoteCopy(t *testing.T) {
	initialNoteTitle := "Title"
	initialNote, _ := NewNote(initialNoteTitle, "Text", nil)
	dupeNote := initialNote.Copy()
	require.Equal(t, initialNote.Content.GetTitle(), initialNoteTitle)
	require.NotNil(t, dupeNote.Content)
	require.Equal(t, initialNote.UUID, dupeNote.UUID)
	require.Equal(t, initialNote.ContentType, dupeNote.ContentType)
	require.Equal(t, initialNote.Content.GetText(), dupeNote.Content.GetText())
	require.Equal(t, initialNote.Content.GetTitle(), dupeNote.Content.GetTitle())
	require.Equal(t, initialNote.ContentSize, dupeNote.ContentSize)
	require.Equal(t, initialNote.CreatedAt, dupeNote.CreatedAt)
	require.Equal(t, initialNote.UpdatedAt, dupeNote.UpdatedAt)
}

func TestTagCopy(t *testing.T) {
	initialTag, _ := NewTag("Title", nil)
	dupeTag := initialTag.Copy()
	require.NotNil(t, dupeTag.Content)
	require.Equal(t, dupeTag.UUID, initialTag.UUID)
	require.Equal(t, dupeTag.ContentType, initialTag.ContentType)
	require.Equal(t, dupeTag.Content.GetTitle(), initialTag.Content.GetTitle())
	require.Equal(t, dupeTag.ContentSize, initialTag.ContentSize)
	require.Equal(t, dupeTag.CreatedAt, initialTag.CreatedAt)
	require.Equal(t, dupeTag.UpdatedAt, initialTag.UpdatedAt)
}

func TestTagComparison(t *testing.T) {
	xUUID := GenUUID()
	one, _ := NewTag("one", nil)
	one.UUID = xUUID
	two, _ := NewTag("one", nil)
	two.UUID = xUUID
	require.True(t, one.Equals(two))

	one.Content.SetTitle("one")
	two.Content.SetTitle("one")
	require.True(t, one.Equals(two))

	one.Content.SetTitle("one")
	two.Content.SetTitle("two")
	require.False(t, one.Equals(two))
}

func TestNoteTagging(t *testing.T) {
	cleanup()
	defer cleanup()

	// create base notes
	newNotes := genNotes(10, 2)
	require.NoError(t, newNotes.Validate(testSession))
	eItems, err := newNotes.Encrypt(testSession, testSession.DefaultItemsKey)
	require.NoError(t, err)
	_, err = Sync(SyncInput{
		Session: testSession,
		Items:   eItems,
	})
	require.NoError(t, err)

	dogNote := createNote("Dogs", "Can't look up", GenUUID())
	cheeseNote := createNote("Cheese", "Is not a vegetable", GenUUID())
	baconNote := createNote("Bacon", "Goes with everything", GenUUID())
	gnuNote := createNote("GNU", "Is not Unix", GenUUID())
	spiderNote := createNote("Spiders", "Are not welcome", GenUUID())
	// tag dog and gnu note with animal tag
	animalFactsTag, _ := createTag("Animal Facts", GenUUID(), nil)

	updatedAnimalTagsInput := UpdateItemRefsInput{
		Items: Items{animalFactsTag},
		ToRef: Items{dogNote, gnuNote, spiderNote},
	}
	updatedAnimalTagsOutput := UpdateItemRefs(updatedAnimalTagsInput)

	// confirm new tags both reference dog and gnu notes
	animalNoteUUIDs := []string{
		dogNote.UUID,
		gnuNote.UUID,
		spiderNote.UUID,
	}

	foodNoteUUIDs := []string{
		cheeseNote.UUID,
		baconNote.UUID,
	}

	// tag cheese note with food tag
	foodFactsTag, _ := createTag("Food Facts", GenUUID(), nil)

	updatedFoodTagsInput := UpdateItemRefsInput{
		Items: Items{foodFactsTag},
		ToRef: Items{cheeseNote, baconNote},
	}
	updatedFoodTagsOutput := UpdateItemRefs(updatedFoodTagsInput)

	for _, at := range updatedAnimalTagsOutput.Items {
		at = at.(*Tag)
		for _, ref := range at.GetContent().References() {
			if !slices.Contains(animalNoteUUIDs, ref.UUID) {
				t.Error("failed to find an animal note reference")
			}

			if slices.Contains(foodNoteUUIDs, ref.UUID) {
				t.Error("found a food note reference")
			}
		}
	}

	for _, ft := range updatedFoodTagsOutput.Items {
		for _, ref := range ft.GetContent().References() {
			if !slices.Contains(foodNoteUUIDs, ref.UUID) {
				t.Error("failed to find an food note reference")
			}

			if slices.Contains(animalNoteUUIDs, ref.UUID) {
				t.Error("found an animal note reference")
			}
		}
	}

	// Put Notes and Tags
	var allItems Items
	allItems = append(allItems, dogNote, cheeseNote, gnuNote, spiderNote, baconNote)
	allItems = append(allItems, updatedAnimalTagsOutput.Items...)
	allItems = append(allItems, updatedFoodTagsOutput.Items...)

	require.NoError(t, allItems.Validate(testSession))
	eItems, err = allItems.Encrypt(testSession, testSession.DefaultItemsKey)
	require.NoError(t, err)

	_, err = Sync(SyncInput{
		Items:   eItems,
		Session: testSession,
	})
	require.NoError(t, err)

	getAnimalNotesFilters := ItemFilters{
		Filters: []Filter{{
			Type:       "Note",
			Key:        "TagTitle",
			Comparison: "~",
			Value:      "Animal Facts",
		}},
		MatchAny: true,
	}

	var so SyncOutput

	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err, "failed to retrieve animal notes by tag")

	var animalNotes Items

	animalNotes, err = so.Items.DecryptAndParse(testSession)
	require.NoError(t, err)

	animalNotes.Filter(getAnimalNotesFilters)
	// check two notes are animal tagged ones
	animalNoteTitles := []string{
		dogNote.Content.GetTitle(),
		gnuNote.Content.GetTitle(),
		spiderNote.Content.GetTitle(),
	}

	if len(animalNotes) != 3 {
		t.Errorf("expected three notes, got: %d", len(animalNotes))
	}

	for _, fn := range animalNotes {
		an := fn.(*Note)
		if !slices.Contains(animalNoteTitles, an.Content.Title) {
			t.Error("got non animal note based on animal tag")
		}
	}

	// get using regex
	regexFilter := Filter{
		Type:       "Note",
		Comparison: "~",
		Key:        "Text",
		Value:      `not\s(Unix|a vegetable)`,
	}

	regexFilters := ItemFilters{
		Filters: []Filter{regexFilter},
	}

	getNotesInput := SyncInput{
		Session: testSession,
	}

	so, err = Sync(getNotesInput)
	require.NoError(t, err, "failed to retrieve notes using regex")

	var notes Items
	notes, err = so.Items.DecryptAndParse(testSession)
	require.NoError(t, err)

	notes.Filter(regexFilters)
	// check two notes are animal tagged ones
	expectedNoteTitles := []string{"Cheese", "GNU"}
	if len(notes) != len(expectedNoteTitles) {
		t.Errorf("expected two notes, got: %d", len(notes))
	}

	for _, fn := range notes {
		an := fn.(*Note)
		if !slices.Contains(expectedNoteTitles, an.Content.Title) {
			t.Errorf("got unexpected result: %s", an.Content.Title)
		}
	}
}

func TestSearchNotesByUUID(t *testing.T) {
	defer cleanup()

	// create two notes
	noteInput := map[string]string{
		"Cheese Fact": "Cheese is not a vegetable",
		"Dog Fact":    "Dogs can't look up",
		"GNU":         "Is Not Unix",
	}

	cnO, err := _createNotes(testSession, noteInput)
	require.NoError(t, err, "failed to create notes")

	var dogFactUUID string

	di, err := DecryptItems(testSession, cnO.SavedItems, []session.SessionItemsKey{})
	require.NoError(t, err)

	dis, err := di.Parse()
	require.NoError(t, err)

	for _, d := range dis {
		dn := d.(*Note)
		if dn.Content.Title == "Dog Fact" {
			dogFactUUID = dn.UUID
		}
	}

	var foundItems Items

	filterOne := Filter{
		Type:  "Note",
		Key:   "UUID",
		Value: dogFactUUID,
	}

	var itemFilters ItemFilters
	itemFilters.Filters = []Filter{filterOne}

	foundItems, err = _getItems(testSession, itemFilters)
	if err != nil {
		t.Error(err.Error())
	}

	// check correct items returned
	switch len(foundItems) {
	case 0:
		t.Errorf("no notes returned")
	case 1:
		fi := foundItems[0].(*Note)
		if fi.Content.Title != "Dog Fact" {
			t.Errorf("incorrect note returned (title mismatch)")
		}

		if !strings.Contains(fi.Content.Text, "Dogs can't look up") {
			t.Errorf("incorrect note returned (text mismatch)")
		}
	default:
		t.Errorf("expected one note but got: %d", len(foundItems))
	}
}

func TestSearchNotesByText(t *testing.T) {
	time.Sleep(2 * time.Second)
	cleanup()

	defer cleanup()

	_, err := Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	// create two notes
	noteInput := map[string]string{
		"Dog Fact":    "Dogs can't look up",
		"Cheese Fact": "Cheese is not a vegetable",
	}

	if _, err = _createNotes(testSession, noteInput); err != nil {
		t.Errorf("failed to create notes")
	}

	// find one note by text
	var foundItems Items

	filterOne := Filter{
		Type:       "Note",
		Key:        "Text",
		Comparison: "contains",
		Value:      "Cheese",
	}

	var itemFilters ItemFilters
	itemFilters.Filters = []Filter{filterOne}

	foundItems, err = _getItems(testSession, itemFilters)
	if err != nil {
		t.Error(err.Error())
	}
	// check correct items returned
	switch len(foundItems) {
	case 0:
		t.Errorf("no notes returned")
	case 1:
		fi := foundItems[0].(*Note)
		if fi.Content.Title != "Cheese Fact" {
			t.Errorf("incorrect note returned (title mismatch)")
		}

		if !strings.Contains(fi.Content.Text, "Cheese is not a vegetable") {
			t.Errorf("incorrect note returned (text mismatch)")
		}
	default:
		t.Errorf("expected one note but got: %d", len(foundItems))
	}
}

func TestSearchNotesByRegexTitleFilter(t *testing.T) {
	defer cleanup()

	_, err := Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	// create two notes
	noteInput := map[string]string{
		"Dog Fact":    "Dogs can't look up",
		"Cheese Fact": "Cheese is not a vegetable",
	}
	if _, err = _createNotes(testSession, noteInput); err != nil {
		t.Errorf("failed to create notes")
	}
	// find one note by text
	var foundItems Items

	filterOne := Filter{
		Type:       "Note",
		Key:        "Title",
		Comparison: "~",
		Value:      "^Do.*",
	}

	var itemFilters ItemFilters

	itemFilters.Filters = []Filter{filterOne}

	foundItems, err = _getItems(testSession, itemFilters)
	if err != nil {
		t.Error(err.Error())
	}
	// check correct items returned
	switch len(foundItems) {
	case 0:
		t.Errorf("no notes returned")
	case 1:
		fi := foundItems[0].(*Note)

		if fi.Content.Title != "Dog Fact" {
			t.Errorf("incorrect note returned (title mismatch)")
		}

		if !strings.Contains(fi.Content.Text, "Dogs can't look up") {
			t.Errorf("incorrect note returned (text mismatch)")
		}
	default:
		t.Errorf("expected one note but got: %d", len(foundItems))
	}
}

func TestSearchTagsByText(t *testing.T) {
	defer cleanup()

	_, err := Sync(SyncInput{
		Session: testSession,
	})

	require.NoError(t, err)

	tagInput := []string{"Rod, Jane", "Zippy, Bungle"}
	if _, err = _createTags(testSession, tagInput); err != nil {
		t.Errorf("failed to create tags")
	}
	// find one note by text
	var foundItems Items

	filterOne := Filter{
		Type:       "Tag",
		Key:        "Title",
		Comparison: "contains",
		Value:      "Bungle",
	}

	var itemFilters ItemFilters
	itemFilters.Filters = []Filter{filterOne}

	foundItems, err = _getItems(testSession, itemFilters)
	if err != nil {
		t.Error(err.Error())
	}
	// check correct items returned
	switch len(foundItems) {
	case 0:
		t.Errorf("no tags returned")
	case 1:
		fi := foundItems[0].(*Tag)

		if fi.Content.Title != "Zippy, Bungle" {
			t.Errorf("incorrect tag returned (title mismatch)")
		}
	default:
		t.Errorf("expected one tag but got: %d", len(foundItems))
	}
}

func TestSearchTagsByRegex(t *testing.T) {
	defer cleanup()

	tagInput := []string{"Rod, Jane", "Zippy, Bungle"}
	if _, err := _createTags(testSession, tagInput); err != nil {
		t.Errorf("failed to create tags")
	}
	// find one note by text
	var foundItems Items

	filterOne := Filter{
		Type:       "Tag",
		Key:        "Title",
		Comparison: "~",
		Value:      "pp",
	}

	var itemFilters ItemFilters
	itemFilters.Filters = []Filter{filterOne}

	foundItems, err := _getItems(testSession, itemFilters)
	if err != nil {
		t.Error(err.Error())
	}
	// check correct items returned
	switch len(foundItems) {
	case 0:
		t.Errorf("no tags returned")
	case 1:
		fi := foundItems[0].(*Tag)
		if fi.Content.Title != "Zippy, Bungle" {
			t.Errorf("incorrect tag returned (title mismatch)")
		}
	default:
		t.Errorf("expected one tag but got: %d", len(foundItems))
	}
}

// create a tag, get its uuid, and then retrieve it by uuid.
func TestSearchItemByUUID(t *testing.T) {
	defer cleanup()

	tagInput := []string{"Bungle"}
	if _, err := _createTags(testSession, tagInput); err != nil {
		t.Errorf("failed to create tags")
	}

	// find one note by text
	var foundItems Items

	filterOne := Filter{
		Type:       "Tag",
		Key:        "Title",
		Comparison: "==",
		Value:      "Bungle",
	}

	var itemFilters ItemFilters
	itemFilters.Filters = []Filter{filterOne}

	foundItems, err := _getItems(testSession, itemFilters)
	if err != nil {
		t.Error(err.Error())
	}
	// check single item returned
	require.Len(t, foundItems, 1)
	filterTwo := Filter{
		Type:       "Item",
		Key:        "uuid",
		Comparison: "==",
		Value:      foundItems[0].GetUUID(),
	}

	itemFilters.Filters = []Filter{filterTwo}

	foundItemsTwo, err := _getItems(testSession, itemFilters)
	if err != nil {
		t.Error(err.Error())
	}

	require.Len(t, foundItemsTwo, 1)
	require.Equal(t, foundItemsTwo[0].GetUUID(), foundItems[0].GetUUID())
}

func genRandomText(paragraphs int) string {
	var strBuilder strings.Builder

	for i := 1; i <= paragraphs; i++ {
		strBuilder.WriteString(testParas[randInt(0, len(testParas))])
	}

	return strBuilder.String()
}

func genNotes(num int, textParas int) (notes Items) {
	for i := 1; i <= num; i++ {
		time.Sleep(2 * time.Millisecond)
		newNote, _ := NewNote(fmt.Sprintf("-%d-,%s", i, "Title"), fmt.Sprintf("%d,%s", i, genRandomText(textParas)), nil)

		notes = append(notes, &newNote)
	}

	return notes
}

// func TestEncryptDecryptOfItemsKey(t *testing.T) {
// 	s := testSession
// 	ik, err := CreateItemsKey()
// 	require.NoError(t, err)
// 	require.Equal(t, common.SNItemTypeItemsKey, ik.ContentType)
// 	require.NotEmpty(t, ik.ItemsKey)
//
// 	eik, err := EncryptItemsKey(ik, testSession, true)
// 	require.NoError(t, err)
// 	require.Equal(t, common.SNItemTypeItemsKey, eik.ContentType)
// 	require.NotEmpty(t, eik.EncItemKey)
//
// 	dik, err := DecryptAndParseItemKeys(s.MasterKey, []EncryptedItem{eik})
// 	require.NoError(t, err)
// 	require.Len(t, dik, 1)
// 	require.Greater(t, len(dik[0].ItemsKey), 0)
// 	require.Equal(t, ik.ItemsKey, dik[0].ItemsKey)
// 	require.NotZero(t, dik[0].CreatedAtTimestamp)
// 	require.Equal(t, ik.CreatedAtTimestamp, dik[0].CreatedAtTimestamp)
// 	require.Equal(t, ik.UpdatedAtTimestamp, dik[0].UpdatedAtTimestamp)
// 	require.Equal(t, ik.CreatedAt, dik[0].CreatedAt)
// 	require.Equal(t, ik.UpdatedAt, dik[0].UpdatedAt)
// }

func TestDecryptNoteText(t *testing.T) {
	// decrypt encrypted items key
	decryptedItemsKey := "366df581a789de771a1613d7d0289bbaff7bf4249a7dd15e458a12c361cb7b73"
	cipherText := "sJhGyLDN4x/wXBcE6TWCsZMaAPfK04ojpsYzjI/zEGvkBsRPGPyihTyQGHvAqcHMWOIZZYZDC2+8YlxVdreF2LblOM8hXz3hwtFDE3DcN5g="
	nonce := "b55df872abe8c97f82bb875a14a9b344584825edef1d0ed7"
	authData := "eyJ1IjoiYmE5MjQ4YWMtOWUxNC00ODcyLTgxNjYtNTkzMjg5ZDg5ODYwIiwidiI6IjAwNCJ9"
	contentItemKeyHexBytes, err := crypto.DecryptCipherText(cipherText, decryptedItemsKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, "b396412f690bfb40801c764af7975bc019f3de79b1ed24385e98787aff81c003", string(contentItemKeyHexBytes))

	rawKey := string(contentItemKeyHexBytes)
	nonce = "6045eaf9774a877203b68bb12159f9c5c0c3d19df4949e40"
	cipherText = "B+8vUwmSTGZCba6mU2gMSMl55fpt38Wv/yWxAF4pEveX0sjqSYgjT5PA8/yy7LKotF+kjmuiHNvYtH7hB7BaqJrG8Q4G5Sj15tIu8PtlWECJWHnPxHkeiJW1MiS1ypR0t3y+Uc7cRpGPwnQIqJDr/Yl1vp2tZXlaSy0zYtGYlw5GwUnLxXtQBQC1Ml3rzZDpaIT9zIr9Qluv7Q7JXOJ7rAbj95MtsV2CJDS33+kXBTUKMqYRbGDWmn0="
	authData = "eyJ1IjoiYmE5MjQ4YWMtOWUxNC00ODcyLTgxNjYtNTkzMjg5ZDg5ODYwIiwidiI6IjAwNCJ9"
	dIKeyContent, err := crypto.DecryptCipherText(cipherText, rawKey, nonce, authData)
	require.NoError(t, err)

	sDiKeyContent := string(dIKeyContent)
	ikc := NoteContent{}
	err = json.Unmarshal([]byte(sDiKeyContent), &ikc)
	require.NoError(t, err)
	require.Equal(t, "Note Title", ikc.Title)
	require.Equal(t, "Note Text", ikc.Text)
}

func TestDecryptItemKey(t *testing.T) {
	// decrypt encrypted item key
	rawKey := "e73faf921cc265b7a001451d8760a6a6e2270d0dbf1668f9971fd75c8018ffd4"
	cipherText := "kRd2w+7FQBIXaNGze7G28GOIUSngrqtx/t5Jus76z3z+eM18GkJT7Lc/ZpqJiH9I6fdksNdo6uvfip8TCIT458XxcrqIP24Bxk9xaz2Q9IQ="
	nonce := "d211fc5dee400fe54ca04ac43ecac512c9d0dabb6c4ee0f3"
	authData := "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	contentItemKeyHexBytes, err := crypto.DecryptCipherText(cipherText, rawKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, "9381f4ac4371cd9e31c3389442897d5c7de3da3d787927709ab601e28767d18a", string(contentItemKeyHexBytes))

	// decrypt item key content with item key
	rawKey = string(contentItemKeyHexBytes)
	nonce = "b0a6519e605db5ecf02cdc225567b61d41f56b41387bc95e"
	cipherText = "GHWwyAayZuu5BKLbHScaJ2e8turXbbcnkNGrmTr9alLQen9UyNRjOtKNH1WcfNb3/kkqabw8XwNxKwrrQwBZmC1wVkIvJpEQc0oI7Nc9F3zHVJyiHqFc8mWRs2jWY+/3IdWm6TTTiJro+QTzFjO5XO9J8KwAx1LizaScjKdTE20p+ryRrrfpp5x8YbbuIWLxpOZRJfF0zUe7wAo/SCI/VuIvSrTK9958VgvPzTagse644pjSo/yvcaSv5XUJhfvaBeqK0JLwiNvNmYZHXt1itfHRE1BFi6/T0fkA30VQb8JmHyHU"
	authData = "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	dIKeyContent, err := crypto.DecryptCipherText(cipherText, rawKey, nonce, authData)
	require.NoError(t, err)

	var ik ItemsKey
	err = json.Unmarshal(dIKeyContent, &ik)
	require.NoError(t, err)
	require.Equal(t, "366df581a789de771a1613d7d0289bbaff7bf4249a7dd15e458a12c361cb7b73", ik.ItemsKey)
}
