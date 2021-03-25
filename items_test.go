package gosn

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	sInput = SignInInput{
		Email:     os.Getenv("SN_EMAIL"),
		Password:  os.Getenv("SN_PASSWORD"),
		APIServer: os.Getenv("SN_SERVER"),
	}
	testParas = []string{
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Ut venenatis est sit amet lectus aliquam, ac rutrum nibh vulputate. Etiam vel nulla dapibus, lacinia neque et, porttitor elit. Nulla scelerisque elit sem, ac posuere est gravida dignissim. Fusce laoreet, enim gravida vehicula aliquam, tellus sem iaculis lorem, rutrum congue ex lectus ut quam. Cras sollicitudin turpis magna, et tempor elit dignissim eu. Etiam sed auctor leo. Sed semper consectetur purus, nec vehicula tellus tristique ac. Cras a quam et magna posuere varius vitae posuere sapien. Morbi tincidunt tellus eu metus laoreet, quis pulvinar sapien consectetur. Fusce nec viverra lectus, sit amet ullamcorper elit. Vestibulum vestibulum augue sem, vitae egestas ipsum fringilla sit amet. Nulla eget ante sit amet velit placerat gravida.",
		"Duis odio tortor, consequat egestas neque dictum, porttitor laoreet felis. Sed sed odio non orci dignissim vulputate. Praesent a scelerisque lectus. Phasellus sit amet vestibulum felis. Integer blandit, nulla eget tempor vestibulum, nisl dolor interdum eros, sed feugiat est augue sit amet eros. Suspendisse maximus tortor sodales dolor sagittis, vitae mattis est cursus. Etiam lobortis nunc non mi posuere, vel elementum massa congue. Aenean ut lectus vitae nisl scelerisque semper.",
		"Quisque pellentesque mauris ut tortor ultrices, eget posuere metus rhoncus. Aenean maximus ultricies mauris vel facilisis. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Curabitur hendrerit, ligula a sagittis condimentum, metus nibh sodales elit, sed rhoncus felis ipsum sit amet sem. Phasellus nec massa non felis suscipit dictum. Aenean dictum iaculis metus quis aliquam. Aenean suscipit mi vel nisi finibus rhoncus. Donec eleifend, massa in convallis mattis, justo eros euismod dui, sollicitudin imperdiet nibh lacus sit amet diam. Praesent eu mollis ligula. In quis nisi egestas, scelerisque ante vitae, dignissim nisi. Curabitur vel est eget purus porta malesuada.",
		"Duis tincidunt eros ligula, et tincidunt lacus scelerisque ac. Cras aliquam ultrices egestas. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Nunc sapien est, imperdiet in cursus id, suscipit ac orci. Integer magna augue, accumsan quis massa rutrum, dictum posuere odio. Vivamus vitae efficitur enim. Donec posuere sapien sit amet turpis lacinia rutrum. Nulla porttitor lacinia justo quis consequat.",
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
		"Pellentesque venenatis elementum enim, nec consequat felis. Nulla facilisi. Suspendisse tempus erat non ipsum vulputate, in lacinia elit feugiat. Maecenas et velit eget tortor congue luctus nec eu urna. Duis congue tellus vel purus convallis tristique. Aenean dapibus tincidunt leo, vel aliquam turpis pretium sit amet. Pellentesque mollis in massa eu mattis. Suspendisse potenti. Cras facilisis at purus ut elementum. Sed volutpat eget nisl id lobortis. Etiam vestibulum lectus id justo vulputate lacinia quis ut lectus. Phasellus tincidunt dolor id nisl placerat, a consequat est volutpat. Phasellus venenatis finibus ante, et eleifend justo pulvinar id. Cras eu ligula quis libero tempor condimentum. Praesent accumsan enim in sodales vulputate. Curabitur rhoncus ante a luctus interdum.",
		"Maecenas vel rhoncus turpis, sit amet varius lacus. Vivamus venenatis sapien vel mi euismod, eget commodo tortor auctor. Cras sit amet dictum quam, non fermentum massa. Aliquam gravida est sed gravida suscipit. Praesent finibus tempor magna, ut dapibus dolor. Proin quis pulvinar arcu. Suspendisse tempor sem justo, at dignissim justo elementum vel. Aenean vitae dolor varius lacus rutrum eleifend.",
		"Etiam ut varius enim. Quisque ligula neque, accumsan et neque eget, pretium lacinia nisi. Etiam aliquet id quam a ullamcorper. Fusce eleifend, mauris vitae placerat egestas, orci erat euismod enim, ut posuere nisl justo placerat libero. Nam ac dui ac lorem laoreet maximus. Curabitur risus leo, feugiat et ligula ac, pellentesque ullamcorper lorem. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae; Donec dignissim non turpis tristique hendrerit. Donec libero odio, ullamcorper condimentum tincidunt ac, hendrerit sed metus. Maecenas venenatis sodales ex. Vestibulum sit amet finibus urna, eu pellentesque velit. Donec accumsan lectus sit amet purus lacinia, et aliquam quam imperdiet. Nunc quis sem fermentum, consectetur urna quis, tristique eros. Sed at tortor a velit blandit aliquam in semper odio. Etiam laoreet sapien lacus, at convallis felis feugiat vitae. Integer et facilisis nibh.",
		"Nullam consequat vehicula euismod. Donec non metus sed nulla bibendum facilisis sed vitae orci. Donec at sapien elit. Sed luctus id augue a gravida. Quisque bibendum nisl sed imperdiet congue. Nam tristique diam diam, ut finibus ante laoreet sit amet. Fusce eget condimentum sem, eget imperdiet massa. Sed orci velit, aliquet a malesuada ac, convallis vitae elit. Aliquam molestie tellus vitae tellus accumsan, quis dapibus purus placerat. Cras commodo, ligula quis commodo congue, ipsum enim placerat nisi, eu congue ante dolor sed ante. Nunc luctus est id metus eleifend, sed consequat leo gravida. Phasellus mattis enim sit amet placerat vehicula. Suspendisse vestibulum lacus sit amet nunc placerat, et ultricies elit fermentum. In et est ac turpis vestibulum bibendum.",
		"Cras in est efficitur, volutpat nisi ut, faucibus nisl. Proin vehicula quam vel ante cursus, vel ultricies erat rutrum. Aliquam pharetra fringilla mauris eget condimentum. Sed placerat turpis eu turpis semper, nec vestibulum sem ornare. Suspendisse potenti. Ut at elit porta, luctus lorem in, tincidunt nunc. Vivamus in justo a lectus sollicitudin tempor non non tellus. Phasellus nec iaculis lacus. Cras consequat orci nec feugiat rutrum. Praesent vulputate, lorem a vulputate ornare, nisi ante tempus elit, sit amet interdum eros nunc vitae erat. Donec at neque in orci ultricies pulvinar eu sit amet quam. Duis tempus vitae sapien vel malesuada.",
		"In auctor condimentum diam sed auctor. Vivamus id rutrum arcu. Proin venenatis, neque malesuada eleifend posuere, urna est pellentesque turpis, vitae scelerisque nisi nunc sit amet ligula. Vestibulum viverra consequat est in lobortis. Nam efficitur arcu at neque lacinia, ut condimentum leo gravida. Vestibulum vitae nisi leo. Nulla odio dolor, malesuada a magna non, accumsan efficitur nisi. Proin vitae lacus facilisis erat interdum pulvinar. Praesent in egestas tortor.", "Integer in massa odio. Duis ex mi, varius nec nibh eget, sodales pellentesque sapien. Aenean pharetra eros vitae tellus volutpat euismod. Vestibulum mattis condimentum metus eget ullamcorper. Integer ut iaculis lorem. Fusce nec lobortis massa. Aliquam pellentesque placerat lorem in efficitur. In pharetra et ex sed mattis. Praesent eget nisi sit amet metus ullamcorper fermentum. Integer ante dolor, vestibulum id massa in, dictum porta nisi. Duis lacinia est vitae enim dignissim blandit. Integer non dapibus velit. Cras auctor eu felis sodales tempor.",
		"Fusce convallis facilisis lacus id lacinia. Nunc rhoncus vulputate consectetur. Ut malesuada malesuada mi, molestie luctus purus feugiat at. Integer eros mauris, porttitor eu tristique quis, gravida id enim. Nullam vitae lectus nulla. Etiam consequat sapien vitae risus volutpat, vitae condimentum nisl ultrices. Donec et magna sed justo suscipit iaculis. Vestibulum id mauris eu eros tempus interdum. Morbi porta libero quis leo consectetur maximus. Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Proin ut dui pharetra, faucibus augue blandit, tempor massa. Duis urna odio, luctus eget justo sed, consectetur faucibus mauris. Phasellus vel sapien justo. Donec sed lectus eget lectus ultrices ornare. Curabitur ultrices sapien libero, sit amet fermentum mi bibendum nec.",
		"Phasellus sed quam mi. Aenean eget sodales neque. Nam convallis lacus non justo blandit, non ornare mauris imperdiet. Nam mattis commodo turpis et lacinia. Duis ac maximus lorem. Suspendisse euismod dui at sodales accumsan. Suspendisse vulputate lobortis sapien, viverra vehicula felis blandit vitae. Fusce viverra ultrices felis sed egestas. Duis ex diam, rutrum nec maximus sit amet, imperdiet ac ex. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Mauris scelerisque facilisis eros, sed maximus erat elementum et.",
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

//
//func _createNotes(session Session, input map[string]string) (so SyncOutput, err error) {
//	var newNotes Items
//
//	for k, v := range input {
//		newNote := NewNote()
//		newNoteContent := NewNoteContent()
//		newNoteContent.Title = k
//		newNoteContent.Text = v
//		newNote.Content = *newNoteContent
//		newNotes = append(newNotes, &newNote)
//	}
//
//	if len(newNotes) > 0 {
//		eNotes, _ := newNotes.Encrypt("", "", true)
//		si := SyncInput{
//			Session: session,
//			Items:   eNotes,
//		}
//
//		so, err = Sync(si)
//		if err != nil {
//			err = fmt.Errorf("PutItems Failed: %v", err)
//		}
//	}
//
//	return
//}
//
//func _createTags(session Session, input []string) (output SyncOutput, err error) {
//	for _, tt := range input {
//		newTag := NewTag()
//		newTagContent := TagContent{
//			Title: tt,
//		}
//		newTagContent.SetUpdateTime(time.Now())
//		newTag.Content = newTagContent
//
//		dItems := Items{&newTag}
//		eItems, _ := dItems.Encrypt("", "", true)
//		si := SyncInput{
//			Session: session,
//			Items:   eItems,
//		}
//		output, err = Sync(si)
//
//		if err != nil {
//			err = fmt.Errorf("PutItems Failed: %v", err)
//			return
//		}
//	}
//
//	return
//}

func _deleteAllTagsNotesComponents(session *Session) (err error) {
	gnf := Filter{
		Type: "Note",
	}
	gtf := Filter{
		Type: "Tag",
	}
	gcf := Filter{
		Type: "SN|Component",
	}

	f := ItemFilters{
		Filters:  []Filter{gnf, gtf, gcf},
		MatchAny: true,
	}
	si := SyncInput{
		Session: *session,
		Debug:   true,
	}

	var so SyncOutput

	so, err = Sync(si)
	if err != nil {
		return
	}

	var iks []ItemsKey
	iks, err = so.Items.DecryptAndParseItemsKeys(session.MasterKey, true)
	var uuids []string

	seenItems := make(map[string]bool)

	var notes, components, tags int

	for _, x := range so.Items {
		if seenItems[x.UUID] {
			panic("got duplicate!!!!!")
		} else {
			seenItems[x.UUID] = true
		}

		if x.Deleted {
			continue
		}

		uuids = append(uuids, x.UUID)

		switch x.ContentType {
		case "Note":
			notes++
		case "Tag":
			tags++
		case "SN|Component":
			components++
		default:
			debugPrint(si.Debug, fmt.Sprintf("ignoring %s", x.ContentType))
		}
	}

	var items Items
	items, err = so.Items.DecryptAndParse(session.MasterKey, iks[0], session.AccessToken, true)
	if err != nil {
		return
	}
	items.Filter(f)

	var toDel Items

	for x := range items {
		md := items[x]
		switch md.GetContentType() {
		case "Note":
			md.SetContent(*NewNoteContent())
		case "Tag":
			md.SetContent(*NewTagContent())
		case "SN|Component":
			md.SetContent(*NewComponentContent())
		}

		md.SetDeleted(true)
		toDel = append(toDel, md)
	}

	if len(toDel) > 0 {
		eToDel, _ := toDel.Encrypt(session.MasterKey, iks[0], true)
		si := SyncInput{
			Session: *session,
			Items:   eToDel,
		}

		_, err = Sync(si)
		if err != nil {
			return fmt.Errorf("PutItems Failed: %v", err)
		}
	}

	return err
}

//func _getItems(session Session, itemFilters ItemFilters) (items Items, err error) {
//	si := SyncInput{
//		Session: session,
//	}
//
//	var so SyncOutput
//
//	so, err = Sync(si)
//	if err != nil {
//		err = fmt.Errorf("sync failed: %v", err)
//		return
//	}
//
//	items, err = so.Items.DecryptAndParse(session.MasterKey, session.AccessToken, true)
//	if err != nil {
//		return
//	}
//
//	items.Filter(itemFilters)
//
//	return
//}
//
//func createNote(title, text, uuid string) *Note {
//	note := NewNote()
//	if uuid != "" {
//		note.UUID = uuid
//	}
//
//	content := NewNoteContent()
//	content.Title = title
//	content.Text = text
//	note.Content = *content
//
//	return &note
//}
//
//func createTag(title, uuid string) *Tag {
//	tag := NewTag()
//	if uuid != "" {
//		tag.UUID = uuid
//	}
//
//	content := NewTagContent()
//	content.Title = title
//	tag.Content = *content
//
//	return &tag
//}

func cleanup(session *Session) {
	if err := _deleteAllTagsNotesComponents(session); err != nil {
		log.Fatal(err)
	}
}

func TestDecryptItemsKeys(t *testing.T) {
	sOutput, err := SignIn(sInput)
	assert.NoError(t, err, "sign-in failed", err)

	defer func() {
		cleanup(&sOutput.Session)
	}()

	syncInput := SyncInput{
		Session: sOutput.Session,
		Debug:   true,
	}

	var syncOutput SyncOutput

	syncOutput, err = Sync(syncInput)
	assert.NoError(t, err, "Sync Failed", err)

	iks, err := syncOutput.Items.DecryptAndParseItemsKeys(sOutput.Session.MasterKey, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, iks)

}

func TestEncryptDecryptItem(t *testing.T) {
	sOutput, err := SignIn(sInput)
	assert.NoError(t, err, "sign-in failed", err)

	// sync to get items keys
	si := SyncInput{
		Session: sOutput.Session,
	}

	var so SyncOutput

	so, err = Sync(si)
	assert.NoError(t, err)

	// get items key
	var iks []ItemsKey
	iks, err = so.Items.DecryptAndParseItemsKeys(sOutput.Session.MasterKey, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, iks)
	assert.Len(t, iks, 1)

	randPara := testParas[randInt(0, len(testParas))]

	newNoteContent := NoteContent{
		Title:          "TestTitle",
		Text:           randPara,
		ItemReferences: nil,
	}

	newNoteContent.SetUpdateTime(time.Now())

	newNote := NewNote()
	newNote.Content = newNoteContent
	dItems := Items{&newNote}
	assert.NoError(t, dItems.Validate())

	var eItems EncryptedItems
	eItems, err = dItems.Encrypt(sOutput.Session.MasterKey, iks[0], true)
	assert.NoError(t, err)
	assert.NotEmpty(t, eItems)

	// Now Decrypt Item
	var items Items
	items, err = eItems.DecryptAndParse(sOutput.Session.MasterKey, iks[0], "", true)
	assert.NoError(t, err)
	assert.NotEmpty(t, items)

}

//func TestDecryptItems(t *testing.T) {
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	defer func() {
//		cleanup(&sOutput.Session)
//	}()
//
//	syncInput := SyncInput{
//		Session: sOutput.Session,
//		Debug:   true,
//	}
//
//	var syncOutput SyncOutput
//
//	syncOutput, err = Sync(syncInput)
//	assert.NoError(t, err, "Sync Failed", err)
//
//	items, err := decryptItems(sOutput.Session.MasterKey, syncOutput.Items, nil)
//	assert.NoError(t, err)
//	assert.NotEmpty(t, items)
//}
//
//func getItemsKey(masterKey, uuid string, keys *[]itemsKey, items EncryptedItems) (ik ItemsKey, err error) {
//	// instanciate key slice if nil pointer passed
//	if keys == nil {
//		keys = &[]itemsKey{}
//	}
//
//	// return previously obtained items key
//	for _, k := range *keys {
//		if uuid == k.uuid {
//			return k, nil
//		}
//	}
//
//	// search items for items key
//	for _, k := range items {
//		if k.UUID == uuid {
//			var ks []itemsKey
//			ks, err = decryptItemKeys(masterKey, EncryptedItems{k})
//			if err != nil {
//				return itemsKey{}, err
//			}
//			*keys = append(*keys, ks[0])
//			return ks[0], err
//		}
//	}
//
//	err = fmt.Errorf("items key with uuid: %s could not be found", uuid)
//
//	return itemsKey{}, err
//}

func (ei *EncryptedItems) GetItemsKeys(mk string) (iks []ItemsKey, err error) {
	var encItemKeys EncryptedItems

	for _, i := range *ei {
		if i.ContentType == "SN|ItemsKey" && strings.HasPrefix(i.Content, "004") {
			encItemKeys = append(encItemKeys, i)
		}
	}

	if encItemKeys == nil {
		return nil, fmt.Errorf("no ItemsKeys found")
	}

	iks, err = encItemKeys.DecryptAndParseItemsKeys(mk, true)

	return iks, err
}

//
func TestPutItemsAddSingleNote(t *testing.T) {
	// SetDebugLogger(log.Println)
	sOutput, err := SignIn(sInput)
	assert.NoError(t, err, "sign-in failed", err)

	//defer cleanup(&sOutput.Session)

	// sync to get items keys
	si := SyncInput{
		Session: sOutput.Session,
	}

	var so SyncOutput

	so, err = Sync(si)
	assert.NoError(t, err)

	var iks []ItemsKey
	iks, err = so.Items.DecryptAndParseItemsKeys(sOutput.Session.MasterKey, true)
	assert.NoError(t, err)
	if len(iks) > 1 {
		err = fmt.Errorf("too many items keys returned")
	}

	randPara := "TestText"

	newNoteContent := NoteContent{
		Title:          "TestTitle",
		Text:           randPara,
		ItemReferences: nil,
	}

	newNoteContent.SetUpdateTime(time.Now())

	newNote := NewNote()
	newNote.Content = newNoteContent
	dItems := Items{&newNote}
	assert.NoError(t, dItems.Validate())
	var eItems EncryptedItems
	eItems, err = dItems.Encrypt(sOutput.Session.MasterKey, iks[0], true)
	assert.NoError(t, err)
	assert.NotEmpty(t, eItems)

	si = SyncInput{
		Items:   eItems,
		Session: sOutput.Session,
	}

	so, err = Sync(si)
	assert.NoError(t, err, "Sync Failed", err)
	assert.Len(t, so.SavedItems, 1, "expected 1")
	uuidOfNewItem := so.SavedItems[0].UUID
	si = SyncInput{
		Session: sOutput.Session,
	}

	so, err = Sync(si)
	if err != nil {
		return
	}

	var di DecryptedItems

	di, err = so.Items.Decrypt(iks[0], sOutput.Session.MasterKey, true)
	if err != nil {
		return
	}

	var items Items
	items, err = di.Parse()
	assert.NoError(t, err, "failed to get items")

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

// func TestPutItemsAddSingleNote(t *testing.T) {
//	// SetDebugLogger(log.Println)
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	//defer cleanup(&sOutput.Session)
//
//	// sync to get items keys
//	si := SyncInput{
//		Session: sOutput.Session,
//	}
//
//	var so SyncOutput
//
//	so, err = Sync(si)
//	assert.NoError(t, err)
//
//	var iks []ItemsKey
//	iks, err = so.Items.DecryptAndParseItemsKeys(sOutput.Session.MasterKey, true)
//	assert.NoError(t, err)
//	if len(iks) > 1 {
//		err = fmt.Errorf("too many items keys returned")
//	}
//
//	//randPara := testParas[randInt(0, len(testParas))]
//	randPara := "TestText"
//
//	newNoteContent := NoteContent{
//		Title:          "TestTitle",
//		Text:           randPara,
//		ItemReferences: nil,
//	}
//
//	newNoteContent.SetUpdateTime(time.Now())
//
//	newNote := NewNote()
//	newNote.Content = newNoteContent
//	dItems := Items{&newNote}
//	assert.NoError(t, dItems.Validate())
//	var eItems EncryptedItems
//	eItems, err = dItems.Encrypt(sOutput.Session.MasterKey, iks[0], true)
//	assert.NoError(t, err)
//	assert.NotEmpty(t, eItems)
//
//	si = SyncInput{
//		Items:   eItems,
//		Session: sOutput.Session,
//	}
//
//	so, err = Sync(si)
//	assert.NoError(t, err, "Sync Failed", err)
//	assert.Len(t, so.SavedItems, 1, "expected 1")
//	uuidOfNewItem := so.SavedItems[0].UUID
//	si = SyncInput{
//		Session: sOutput.Session,
//	}
//
//	so, err = Sync(si)
//	if err != nil {
//		return
//	}
//
//	var di DecryptedItems
//
//	di, err = so.Items.Decrypt(iks[0], sOutput.Session.MasterKey, true)
//	if err != nil {
//		return
//	}
//
//	var items Items
//	items, err = di.Parse()
//	assert.NoError(t, err, "failed to get items")
//
//	var foundCreatedItem bool
//
//	for i := range items {
//		if items[i].GetUUID() == uuidOfNewItem {
//			foundCreatedItem = true
//
//			ni := items[i].(*Note)
//
//			if ni.ContentType != "Note" {
//				t.Errorf("content type of new item is incorrect - expected: Note got: %s",
//					items[i].GetContentType())
//			}
//
//			if ni.Deleted {
//				t.Errorf("deleted status of new item is incorrect - expected: False got: True")
//			}
//
//			if ni.Content.GetText() != randPara {
//				t.Errorf("text of new item is incorrect - expected: %s got: %s",
//					randPara, ni.Content.GetText())
//			}
//		}
//	}
//
//	if !foundCreatedItem {
//		t.Errorf("failed to get created Item by UUID")
//	}
//}

//func TestPutItemsAddSingleComponent(t *testing.T) {
//	fmt.Println("In TestPutItemsAddSingleComponent")
//	sOutput, err := SignIn(sInput)
//	fmt.Printf("sOutput: %+v\n", sOutput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	defer cleanup(&sOutput.Session)
//
//	newComponentContent := ComponentContent{
//		Name:               "Minimal Markdown Editor",
//		Area:               "editor-editor",
//		LocalURL:           "sn://Extensions/org.standardnotes.plus-editor/index.html",
//		HostedURL:          "https://extensions.standardnotes.org/e6d4d59ac829ed7ec24e2c139e7d8b21b625dff2d7f98bb7b907291242d31fcd/components/plus-editor",
//		OfflineOnly:        "",
//		ValidUntil:         "2023-08-29T12:15:17.000Z",
//		AutoUpdateDisabled: "",
//		DissociatedItemIds: []string{"e9d4daf5-52e6-4d67-975e-a1620bf5217c"},
//		AssociatedItemIds:  []string{"d7d1dee3-42f6-3d27-871e-d2320bf3214a"},
//		ItemReferences:     nil,
//		Active:             true,
//		AppData:            AppDataContent{},
//	}
//
//	newComponentContent.SetUpdateTime(time.Now())
//
//	newComponent := NewComponent()
//	newComponent.Content = newComponentContent
//
//	newComponent.Content.DisassociateItems([]string{"d7d1dee3-42f6-3d27-871e-d2320bf3214a"})
//	assert.NotContains(t, newComponent.Content.GetItemAssociations(), "d7d1dee3-42f6-3d27-871e-d2320bf3214a")
//
//	newComponent.Content.AssociateItems([]string{"d7d1dee3-42f6-3d27-871e-d2320bf3214a"})
//	assert.Contains(t, newComponent.Content.GetItemAssociations(), "d7d1dee3-42f6-3d27-871e-d2320bf3214a")
//
//	dItems := Items{&newComponent}
//	assert.NoError(t, dItems.Validate())
//	eItems, _ := dItems.Encrypt("", "", true)
//	syncInput := SyncInput{
//		Items:   eItems,
//		Session: sOutput.Session,
//		Debug:   true,
//	}
//
//	var syncOutput SyncOutput
//
//	syncOutput, err = Sync(syncInput)
//	assert.NoError(t, err, "PutItems Failed", err)
//	assert.Len(t, syncOutput.SavedItems, 1, "expected 1")
//	assert.Equal(t, syncInput.Items[0].UUID, syncOutput.SavedItems[0].UUID, "expected 1")
//	uuidOfNewItem := syncOutput.SavedItems[0].UUID
//	syncInput = SyncInput{
//		Session: sOutput.Session,
//		Debug:   true,
//	}
//
//	syncOutput, err = Sync(syncInput)
//	if err != nil {
//		return
//	}
//
//	var di DecryptedItems
//	di, err = syncOutput.Items.Decrypt("", "", true)
//
//	if err != nil {
//		return
//	}
//
//	var items Items
//	items, err = di.Parse()
//
//	var foundCreatedItem bool
//
//	for i := range items {
//		if items[i].GetUUID() == uuidOfNewItem {
//			foundCreatedItem = true
//
//			assert.Equal(t, "SN|Component", items[i].GetContentType())
//			assert.Equal(t, false, items[i].IsDeleted())
//			assert.Equal(t, "Minimal Markdown Editor", items[i].(*Component).Content.GetName())
//		}
//	}
//
//	assert.True(t, foundCreatedItem, "failed to get created Item by UUID")
//}
//
//func TestItemsRemoveDeleted(t *testing.T) {
//	noteTitle := "Title"
//	noteText := "Title"
//	noteContent := NewNoteContent()
//	noteContent.Title = noteTitle
//	noteContent.Text = noteText
//
//	noteOne := NewNote()
//	noteOne.Content = *noteContent
//	noteTwo := noteOne.Copy()
//	noteTwo.UUID += "a"
//	noteThree := noteOne.Copy()
//	noteThree.UUID += "b"
//
//	assert.False(t, noteOne.Deleted)
//
//	assert.False(t, noteTwo.Deleted)
//
//	assert.False(t, noteThree.Deleted)
//
//	noteTwo.Deleted = true
//	notes := Notes{noteOne, noteTwo, noteThree}
//	assert.Len(t, notes, 3)
//	notes.RemoveDeleted()
//	assert.Len(t, notes, 2)
//
//	for _, n := range notes {
//		assert.NotEqual(t, n.UUID, noteTwo.UUID)
//	}
//}
//
//func TestDecryptedItemsRemoveDeleted(t *testing.T) {
//	diOne := DecryptedItem{
//		UUID:        "1234",
//		Content:     "abcd",
//		ContentType: "Note",
//		Deleted:     false,
//	}
//	diTwo := DecryptedItem{
//		UUID:        "2345",
//		Content:     "abcd",
//		ContentType: "Note",
//		Deleted:     true,
//	}
//	diThree := DecryptedItem{
//		UUID:        "3456",
//		Content:     "abcd",
//		ContentType: "Note",
//		Deleted:     false,
//	}
//	dis := DecryptedItems{diOne, diTwo, diThree}
//	assert.Len(t, dis, 3)
//	dis.RemoveDeleted()
//	assert.Len(t, dis, 2)
//
//	for _, n := range dis {
//		assert.NotEqual(t, n.UUID, diTwo.UUID)
//	}
//}
//
//func TestEncryptedItemsRemoveDeleted(t *testing.T) {
//	noteTitle := "Title"
//	noteText := "Title"
//	noteContent := NewNoteContent()
//	noteContent.Title = noteTitle
//	noteContent.Text = noteText
//
//	noteOne := NewNote()
//	noteOne.SetContent(*noteContent)
//	noteTwo := noteOne.Copy()
//	noteTwo.UUID += "a"
//	noteThree := noteOne.Copy()
//	noteThree.UUID += "b"
//
//	assert.False(t, noteOne.Deleted)
//	assert.False(t, noteTwo.Deleted)
//	assert.False(t, noteThree.Deleted)
//
//	noteTwo.Deleted = true
//	notes := Items{&noteOne, &noteTwo, &noteThree}
//
//	var testInput generateEncryptedPasswordInput
//	testInput.userPassword = "oWB7c&77Zahw8XK$AUy#"
//	testInput.Identifier = "soba@lessknown.co.uk"
//	testInput.PasswordNonce = "9e88fc67fb8b1efe92deeb98b5b6a801c78bdfae08eecb315f843f6badf60aef"
//	testInput.PasswordCost = 110000
//	testInput.Version = "003"
//	testInput.PasswordSalt = ""
//	_, mk, ak, err := generateEncryptedPasswordAndKeys(testInput)
//	assert.NoError(t, err)
//
//	var eNotes EncryptedItems
//	eNotes, err = notes.Encrypt(mk, ak, true)
//	assert.NoError(t, err)
//	assert.Len(t, eNotes, 3)
//	eNotes.RemoveDeleted()
//	assert.Len(t, eNotes, 2)
//
//	for _, n := range eNotes {
//		assert.NotEqual(t, n.UUID, noteTwo.UUID)
//	}
//}
//
//func TestNoteContentCopy(t *testing.T) {
//	initialNoteTitle := "Title"
//	initialNoteText := "Title"
//	initialNoteContent := NewNoteContent()
//	initialNoteContent.Title = initialNoteTitle
//	initialNoteContent.Text = initialNoteText
//	dupeNoteContent := initialNoteContent.Copy()
//	// update initial to ensure copy
//	initialNoteContent.Title = "Updated Title"
//	initialNoteContent.Text = "Updated Text"
//	// now check duplicate is correct
//	assert.NotNil(t, dupeNoteContent)
//	assert.Equal(t, initialNoteTitle, dupeNoteContent.Title)
//	assert.Equal(t, initialNoteText, dupeNoteContent.GetText())
//}
//
//func TestTagContentCopy(t *testing.T) {
//	initialTagTitle := "Title"
//	initialTagContent := NewNoteContent()
//	initialTagContent.Title = initialTagTitle
//	dupeNoteContent := initialTagContent.Copy()
//	// update initial to ensure copy
//	initialTagContent.Title = "Updated Title"
//	// now check duplicate is correct
//	assert.NotNil(t, dupeNoteContent)
//	assert.Equal(t, initialTagTitle, dupeNoteContent.Title)
//}
//
//func TestNoteCopy(t *testing.T) {
//	initialNoteTitle := "Title"
//	initialNote := NewNote()
//	initialNoteContent := NewNoteContent()
//	initialNoteContent.Title = initialNoteTitle
//	initialNoteContent.Text = "Text"
//	initialNote.SetContent(*initialNoteContent)
//	dupeNote := initialNote.Copy()
//	assert.Equal(t, initialNote.Content.GetTitle(), initialNoteTitle)
//	assert.NotNil(t, dupeNote.Content)
//	assert.Equal(t, initialNote.UUID, dupeNote.UUID)
//	assert.Equal(t, initialNote.ContentType, dupeNote.ContentType)
//	assert.Equal(t, initialNote.Content.GetText(), dupeNote.Content.GetText())
//	assert.Equal(t, initialNote.Content.GetTitle(), dupeNote.Content.GetTitle())
//	assert.Equal(t, initialNote.ContentSize, dupeNote.ContentSize)
//	assert.Equal(t, initialNote.CreatedAt, dupeNote.CreatedAt)
//	assert.Equal(t, initialNote.UpdatedAt, dupeNote.UpdatedAt)
//}
//
//func TestTagCopy(t *testing.T) {
//	initialTag := NewTag()
//	initialTagContent := NewTagContent()
//	initialTagContent.Title = "Title"
//	initialTag.SetContent(*initialTagContent)
//	dupeTag := initialTag.Copy()
//	assert.NotNil(t, dupeTag.Content)
//	assert.Equal(t, dupeTag.UUID, initialTag.UUID)
//	assert.Equal(t, dupeTag.ContentType, initialTag.ContentType)
//	assert.Equal(t, dupeTag.Content.GetTitle(), initialTag.Content.GetTitle())
//	assert.Equal(t, dupeTag.ContentSize, initialTag.ContentSize)
//	assert.Equal(t, dupeTag.CreatedAt, initialTag.CreatedAt)
//	assert.Equal(t, dupeTag.UpdatedAt, initialTag.UpdatedAt)
//}
//
//func TestTagComparison(t *testing.T) {
//	xUUID := GenUUID()
//	one := NewTag()
//	one.UUID = xUUID
//	one.Content = *NewTagContent()
//	two := NewTag()
//	two.Content = *NewTagContent()
//	two.UUID = xUUID
//	assert.True(t, one.Equals(two))
//
//	one.Content.SetTitle("one")
//	two.Content.SetTitle("one")
//	assert.True(t, one.Equals(two))
//
//	one.Content.SetTitle("one")
//	two.Content.SetTitle("two")
//	assert.False(t, one.Equals(two))
//}
//

//
//func TestNoteTagging(t *testing.T) {
//	sOutput, err := SignIn(sInput)
//
//	assert.NoError(t, err, "sign-in failed", err)
//
//	defer cleanup(&sOutput.Session)
//
//	// create base notes
//	newNotes := genNotes(100, 2)
//	assert.NoError(t, newNotes.Validate())
//	eItems, _ := newNotes.Encrypt("", "", true)
//
//	si := SyncInput{
//		Session: sOutput.Session,
//		Items:   eItems,
//	}
//
//	_, err = Sync(si)
//	if err != nil {
//		t.Errorf(err.Error())
//	}
//
//	dogNote := createNote("Dogs", "Can't look up", "")
//	cheeseNote := createNote("Cheese", "Is not a vegetable", "")
//	baconNote := createNote("Bacon", "Goes with everything", "")
//	gnuNote := createNote("GNU", "Is not Unix", "")
//	spiderNote := createNote("Spiders", "Are not welcome", "")
//
//	animalTag := createTag("Animal Facts", "")
//	foodTag := createTag("Food Facts", "")
//
//	// tag dog and gnu note with animal tag
//	updatedAnimalTagsInput := UpdateItemRefsInput{
//		Items: Items{animalTag},
//		ToRef: Items{dogNote, gnuNote, spiderNote},
//	}
//	updatedAnimalTagsOutput := UpdateItemRefs(updatedAnimalTagsInput)
//
//	// confirm new tags both reference dog and gnu notes
//	animalNoteUUIDs := []string{
//		dogNote.UUID,
//		gnuNote.UUID,
//		spiderNote.UUID,
//	}
//
//	foodNoteUUIDs := []string{
//		cheeseNote.UUID,
//		baconNote.UUID,
//	}
//
//	// tag cheese note with food tag
//	updatedFoodTagsInput := UpdateItemRefsInput{
//		Items: Items{foodTag},
//		ToRef: Items{cheeseNote, baconNote},
//	}
//	updatedFoodTagsOutput := UpdateItemRefs(updatedFoodTagsInput)
//
//	for _, at := range updatedAnimalTagsOutput.Items {
//		at = at.(*Tag)
//		for _, ref := range at.GetContent().References() {
//			if !stringInSlice(ref.UUID, animalNoteUUIDs, true) {
//				t.Error("failed to find an animal note reference")
//			}
//
//			if stringInSlice(ref.UUID, foodNoteUUIDs, true) {
//				t.Error("found a food note reference")
//			}
//		}
//	}
//
//	for _, ft := range updatedFoodTagsOutput.Items {
//		for _, ref := range ft.GetContent().References() {
//			if !stringInSlice(ref.UUID, foodNoteUUIDs, true) {
//				t.Error("failed to find an food note reference")
//			}
//
//			if stringInSlice(ref.UUID, animalNoteUUIDs, true) {
//				t.Error("found an animal note reference")
//			}
//		}
//	}
//
//	// Put Notes and Tags
//	var allItems Items
//	allItems = append(allItems, dogNote, cheeseNote, gnuNote)
//	allItems = append(allItems, updatedAnimalTagsOutput.Items...)
//	allItems = append(allItems, updatedFoodTagsOutput.Items...)
//	assert.NoError(t, allItems.Validate())
//	eItems, _ = allItems.Encrypt("", "", true)
//	si = SyncInput{
//		Items:   eItems,
//		Session: sOutput.Session,
//	}
//
//	_, err = Sync(si)
//	if err != nil {
//		t.Errorf("failed to put items: %+v", err)
//	}
//
//	getAnimalNotesFilter := Filter{
//		Type:       "Note",
//		Key:        "TagTitle",
//		Comparison: "~",
//		Value:      "Animal Facts",
//	}
//	getAnimalNotesFilters := ItemFilters{
//		Filters: []Filter{getAnimalNotesFilter},
//	}
//	getAnimalNotesInput := SyncInput{
//		Session: sOutput.Session,
//	}
//
//	var so SyncOutput
//
//	so, err = Sync(getAnimalNotesInput)
//	if err != nil {
//		t.Error("failed to retrieve animal notes by tag")
//		return
//	}
//
//	var animalNotes Items
//
//	animalNotes, err = so.Items.DecryptAndParse("", "", true)
//	if err != nil {
//		return
//	}
//
//	animalNotes.Filter(getAnimalNotesFilters)
//	// check two notes are animal tagged ones
//	animalNoteTitles := []string{
//		dogNote.Content.GetTitle(),
//		gnuNote.Content.GetTitle(),
//	}
//
//	if len(animalNotes) != 2 {
//		t.Errorf("expected two notes, got: %d", len(animalNotes))
//	}
//
//	for _, fn := range animalNotes {
//		an := fn.(*Note)
//		if !stringInSlice(an.Content.Title, animalNoteTitles, true) {
//			t.Error("got non animal note based on animal tag")
//		}
//	}
//
//	// get using regex
//	regexFilter := Filter{
//		Type:       "Note",
//		Comparison: "~",
//		Key:        "Text",
//		Value:      `not\s(Unix|a vegetable)`,
//	}
//
//	regexFilters := ItemFilters{
//		Filters: []Filter{regexFilter},
//	}
//
//	getNotesInput := SyncInput{
//		Session: sOutput.Session,
//	}
//
//	so, err = Sync(getNotesInput)
//	assert.NoError(t, err, "failed to retrieve notes using regex")
//
//	var notes Items
//	notes, err = so.Items.DecryptAndParse("", "", true)
//	assert.NoError(t, err)
//
//	notes.Filter(regexFilters)
//	// check two notes are animal tagged ones
//	expectedNoteTitles := []string{"Cheese", "GNU"}
//	if len(notes) != len(expectedNoteTitles) {
//		t.Errorf("expected two notes, got: %d", len(notes))
//	}
//
//	for _, fn := range notes {
//		an := fn.(*Note)
//		if !stringInSlice(an.Content.Title, expectedNoteTitles, true) {
//			t.Errorf("got unexpected result: %s", an.Content.Title)
//		}
//	}
//}
//
//func TestSearchNotesByUUID(t *testing.T) {
//	//SetDebugLogger(log.Println)
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	defer cleanup(&sOutput.Session)
//
//	// create two notes
//	noteInput := map[string]string{
//		"Cheese Fact": "Cheese is not a vegetable",
//		"Dog Fact":    "Dogs can't look up",
//		"GNU":         "Is Not Unix",
//	}
//
//	var cnO SyncOutput
//	cnO, err = _createNotes(sOutput.Session, noteInput)
//	assert.NoError(t, err, "failed to create notes")
//
//	var dogFactUUID string
//
//	var di DecryptedItems
//
//	di, err = cnO.SavedItems.Decrypt("", "", true)
//	assert.NoError(t, err)
//
//	var dis Items
//	dis, err = di.Parse()
//	assert.NoError(t, err)
//
//	for _, di := range dis {
//		dn := di.(*Note)
//		if dn.Content.Title == "Dog Fact" {
//			dogFactUUID = dn.UUID
//		}
//	}
//
//	var foundItems Items
//
//	filterOne := Filter{
//		Type:  "Note",
//		Key:   "UUID",
//		Value: dogFactUUID,
//	}
//
//	var itemFilters ItemFilters
//	itemFilters.Filters = []Filter{filterOne}
//
//	foundItems, err = _getItems(sOutput.Session, itemFilters)
//	if err != nil {
//		t.Error(err.Error())
//	}
//
//	// check correct items returned
//	switch len(foundItems) {
//	case 0:
//		t.Errorf("no notes returned")
//	case 1:
//		fi := foundItems[0].(*Note)
//		if fi.Content.Title != "Dog Fact" {
//			t.Errorf("incorrect note returned (title mismatch)")
//		}
//
//		if !strings.Contains(fi.Content.Text, "Dogs can't look up") {
//			t.Errorf("incorrect note returned (text mismatch)")
//		}
//	default:
//		t.Errorf("expected one note but got: %d", len(foundItems))
//	}
//}
//
//func TestSearchNotesByText(t *testing.T) {
//	//SetDebugLogger(log.Println)
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	defer cleanup(&sOutput.Session)
//
//	// create two notes
//	noteInput := map[string]string{
//		"Dog Fact":    "Dogs can't look up",
//		"Cheese Fact": "Cheese is not a vegetable",
//	}
//
//	if _, err = _createNotes(sOutput.Session, noteInput); err != nil {
//		t.Errorf("failed to create notes")
//	}
//	// find one note by text
//	var foundItems Items
//
//	filterOne := Filter{
//		Type:       "Note",
//		Key:        "Text",
//		Comparison: "contains",
//		Value:      "Cheese",
//	}
//
//	var itemFilters ItemFilters
//	itemFilters.Filters = []Filter{filterOne}
//
//	foundItems, err = _getItems(sOutput.Session, itemFilters)
//	if err != nil {
//		t.Error(err.Error())
//	}
//	// check correct items returned
//	switch len(foundItems) {
//	case 0:
//		t.Errorf("no notes returned")
//	case 1:
//		fi := foundItems[0].(*Note)
//		if fi.Content.Title != "Cheese Fact" {
//			t.Errorf("incorrect note returned (title mismatch)")
//		}
//
//		if !strings.Contains(fi.Content.Text, "Cheese is not a vegetable") {
//			t.Errorf("incorrect note returned (text mismatch)")
//		}
//	default:
//		t.Errorf("expected one note but got: %d", len(foundItems))
//	}
//}
//
//func TestSearchNotesByRegexTitleFilter(t *testing.T) {
//	//SetDebugLogger(log.Println)
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	defer cleanup(&sOutput.Session)
//
//	// create two notes
//	noteInput := map[string]string{
//		"Dog Fact":    "Dogs can't look up",
//		"Cheese Fact": "Cheese is not a vegetable",
//	}
//	if _, err = _createNotes(sOutput.Session, noteInput); err != nil {
//		t.Errorf("failed to create notes")
//	}
//	// find one note by text
//	var foundItems Items
//
//	filterOne := Filter{
//		Type:       "Note",
//		Key:        "Title",
//		Comparison: "~",
//		Value:      "^Do.*",
//	}
//
//	var itemFilters ItemFilters
//
//	itemFilters.Filters = []Filter{filterOne}
//
//	foundItems, err = _getItems(sOutput.Session, itemFilters)
//	if err != nil {
//		t.Error(err.Error())
//	}
//	// check correct items returned
//	switch len(foundItems) {
//	case 0:
//		t.Errorf("no notes returned")
//	case 1:
//		fi := foundItems[0].(*Note)
//
//		if fi.Content.Title != "Dog Fact" {
//			t.Errorf("incorrect note returned (title mismatch)")
//		}
//
//		if !strings.Contains(fi.Content.Text, "Dogs can't look up") {
//			t.Errorf("incorrect note returned (text mismatch)")
//		}
//	default:
//		t.Errorf("expected one note but got: %d", len(foundItems))
//	}
//}
//
//func TestSearchTagsByText(t *testing.T) {
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err)
//	cleanup(&sOutput.Session)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	defer cleanup(&sOutput.Session)
//
//	tagInput := []string{"Rod, Jane", "Zippy, Bungle"}
//	if _, err = _createTags(sOutput.Session, tagInput); err != nil {
//		t.Errorf("failed to create tags")
//	}
//	// find one note by text
//	var foundItems Items
//
//	filterOne := Filter{
//		Type:       "Tag",
//		Key:        "Title",
//		Comparison: "contains",
//		Value:      "Bungle",
//	}
//
//	var itemFilters ItemFilters
//	itemFilters.Filters = []Filter{filterOne}
//
//	foundItems, err = _getItems(sOutput.Session, itemFilters)
//	if err != nil {
//		t.Error(err.Error())
//	}
//	// check correct items returned
//	switch len(foundItems) {
//	case 0:
//		t.Errorf("no tags returned")
//	case 1:
//		fi := foundItems[0].(*Tag)
//
//		if fi.Content.Title != "Zippy, Bungle" {
//			t.Errorf("incorrect tag returned (title mismatch)")
//		}
//	default:
//		t.Errorf("expected one tag but got: %d", len(foundItems))
//	}
//}
//
//func TestSearchTagsByRegex(t *testing.T) {
//	//SetDebugLogger(log.Println)
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	cleanup(&sOutput.Session)
//
//	tagInput := []string{"Rod, Jane", "Zippy, Bungle"}
//	if _, err = _createTags(sOutput.Session, tagInput); err != nil {
//		t.Errorf("failed to create tags")
//	}
//	// find one note by text
//	var foundItems Items
//
//	filterOne := Filter{
//		Type:       "Tag",
//		Key:        "Title",
//		Comparison: "~",
//		Value:      "pp",
//	}
//
//	var itemFilters ItemFilters
//	itemFilters.Filters = []Filter{filterOne}
//
//	foundItems, err = _getItems(sOutput.Session, itemFilters)
//	if err != nil {
//		t.Error(err.Error())
//	}
//	// check correct items returned
//	switch len(foundItems) {
//	case 0:
//		t.Errorf("no tags returned")
//	case 1:
//		fi := foundItems[0].(*Tag)
//		if fi.Content.Title != "Zippy, Bungle" {
//			t.Errorf("incorrect tag returned (title mismatch)")
//		}
//	default:
//		t.Errorf("expected one tag but got: %d", len(foundItems))
//	}
//}

//
//func TestCreateAndGet200NotesInBatchesOf50(t *testing.T) {
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//	defer cleanup(&sOutput.Session)

	//newNotes := genNotes(200, 2)
	//assert.NoError(t, newNotes.Validate())
	//
	//var so SyncOutput
	//
	//so, err = Sync(SyncInput{
	//	Session: sOutput.Session,
	//})
	//assert.NoError(t, err)
	//
	//// get items key
	//var iks []ItemsKey
	//iks, err = so.Items.DecryptAndParseItemsKeys(sOutput.Session.MasterKey, true)
	//assert.NoError(t, err)
	//
	//eItems, err := newNotes.Encrypt(sOutput.Session.MasterKey, iks[0], true)
	//assert.NoError(t, err)
	//assert.NoError(t, eItems.Validate())
	//
	//si := SyncInput{
	//	Session: sOutput.Session,
	//	Items:   eItems,
	//	Debug:   true,
	//}
	//
	//_, err = Sync(si)
	//if err != nil {
	//	t.Errorf(err.Error())
	//}
	//
	//var retrievedNotes Items
	//
	//var cursorToken string
	//
	//for {
	//	si = SyncInput{
	//		Session:     sOutput.Session,
	//		CursorToken: cursorToken,
	//		BatchSize:   50,
	//		Debug:       true,
	//	}
	//
	//	so, err = Sync(si)
	//	assert.NoError(t, err)
	//
	//	var di DecryptedItems
	//	di, err = so.Items.Decrypt(iks[0], sOutput.Session.MasterKey, true)
	//	assert.NoError(t, err)
	//
	//	var items Items
	//
	//	items, err = di.Parse()
	//	assert.NoError(t, err)
	//
	//	retrievedNotes = append(retrievedNotes, items...)
	//
	//	if stripLineBreak(so.Cursor) == "" {
	//		break
	//	} else {
	//		cursorToken = so.Cursor
	//	}
	//}
	//retrievedNotes.DeDupe()
	//
	//giFilter := Filter{
	//	Type:  "Note",
	//	Key:   "Deleted",
	//	Value: "False",
	//}
	//giFilters := ItemFilters{
	//	Filters: []Filter{giFilter},
	//}
	//
	//retrievedNotes.Filter(giFilters)
	//
	//if len(retrievedNotes) != 200 {
	//	t.Errorf("expected 200 items but got %d\n", len(retrievedNotes))
	//}
//}

//func TestCreateAndGet301Notes(t *testing.T) {
//	numNotes := 301
//	sOutput, err := SignIn(sInput)
//	assert.NoError(t, err, "sign-in failed", err)
//
//	cleanup(&sOutput.Session)
//	defer cleanup(&sOutput.Session)
//
//	newNotes := genNotes(numNotes, 10)
//	assert.NoError(t, newNotes.Validate())
//	eItems, _ := newNotes.Encrypt("", "", true)
//	si := SyncInput{
//		Session: sOutput.Session,
//		Items:   eItems,
//		Debug:   true,
//	}
//
//	_, err = Sync(si)
//	assert.NoError(t, err)
//
//	var retrievedNotes Items
//
//	var cursorToken string
//
//	giFilter := Filter{
//		Type:  "Note",
//		Key:   "Deleted",
//		Value: "False",
//	}
//	giFilters := ItemFilters{
//		Filters: []Filter{giFilter},
//	}
//
//	for {
//		si = SyncInput{
//			Session:     sOutput.Session,
//			CursorToken: cursorToken,
//			Debug:       true,
//		}
//
//		var so SyncOutput
//
//		so, err = Sync(si)
//		if err != nil {
//			t.Error(err)
//		}
//
//		so.Items.DeDupe()
//
//		var items Items
//
//		items, err = so.Items.DecryptAndParse("", "", true)
//		if err != nil {
//			t.Error(err)
//		}
//
//		retrievedNotes = append(retrievedNotes, items...)
//
//		if stripLineBreak(so.Cursor) == "" {
//			break
//		} else {
//			cursorToken = so.Cursor
//		}
//	}
//	retrievedNotes.Filter(giFilters)
//
//	if len(retrievedNotes) != numNotes {
//		t.Errorf("expected %d items but got %d\n", numNotes, len(retrievedNotes))
//	}
//
//	retrievedNotes.Filter(giFilters)
//
//	for i, r := range retrievedNotes {
//		fi := r.(*Note)
//		if !strings.HasPrefix(fi.Content.Title, fmt.Sprintf("-%d-", i+1)) {
//			t.Errorf("incorrect note returned")
//		}
//	}
//}

func genRandomText(paragraphs int) string {
	var strBuilder strings.Builder

	for i := 1; i <= paragraphs; i++ {
		strBuilder.WriteString(testParas[randInt(0, len(testParas))])
	}

	return strBuilder.String()
}

func genNotes(num int, textParas int) (notes Items) {
	for i := 1; i <= num; i++ {
		time.Sleep(3 * time.Millisecond)

		noteContent := &NoteContent{
			Title:          fmt.Sprintf("-%d-,%s", i, "Title"),
			Text:           "test",
			//Text:           fmt.Sprintf("%d,%s", i, genRandomText(textParas)),
			ItemReferences: ItemReferences{},
		}
		noteContent.SetUpdateTime(time.Now())

		newNote := NewNote()

		newNote.Content = *noteContent

		notes = append(notes, &newNote)
	}

	return notes
}
